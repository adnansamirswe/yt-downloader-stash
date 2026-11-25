package engine

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"stash-backend/internal/config"
)

// Tooling holds resolved paths and detected versions of required media binaries.
type Tooling struct {
	YtDlpPath     string
	YtDlpVersion  string
	FFmpegPath    string
	FFmpegVersion string
	JSRuntimeArg  string // e.g. "deno" or "node:/usr/bin/node"
	JSRuntimeName string // e.g. "deno", "node", "none"
	PotURL        string
	CookiesFile   string
	CookiesStatus string
	ProxyURL      string

	potMu        sync.Mutex
	potLastCheck time.Time
	potHealthy   bool

	proxyMu        sync.Mutex
	proxyLastCheck time.Time
	proxyHealthy   bool
}

// DetectTooling resolves yt-dlp, ffmpeg, and JS runtime binaries once at startup.
func DetectTooling(ctx context.Context, cfg config.Config) *Tooling {
	t := &Tooling{
		PotURL:        cfg.PotProviderURL,
		CookiesFile:   cfg.CookiesFile,
		CookiesStatus: "not_set",
		ProxyURL:      cfg.ProxyURL,
	}
	if t.CookiesFile != "" {
		if st, err := os.Stat(t.CookiesFile); err == nil && !st.IsDir() && st.Size() > 0 {
			t.CookiesStatus = "ok"
		} else {
			t.CookiesStatus = "file_missing"
		}
	}

	t.YtDlpPath = resolveBinary(cfg.YtDlpBin, "yt-dlp", true)
	t.FFmpegPath = resolveBinary(cfg.FFmpegBin, "ffmpeg", false)
	t.JSRuntimeName, t.JSRuntimeArg = resolveJSRuntime(cfg.JSRuntime)

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if t.YtDlpPath != "" {
		if out, err := runSimple(probeCtx, t.YtDlpPath, "--version"); err == nil {
			t.YtDlpVersion = strings.TrimSpace(out)
		}
	}
	if t.FFmpegPath != "" {
		if out, err := runSimple(probeCtx, t.FFmpegPath, "-version"); err == nil {
			firstLine, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
			t.FFmpegVersion = strings.TrimSpace(firstLine)
		}
	}

	return t
}

// HasCookies returns true when a readable, non-empty cookie file is active.
func (t *Tooling) HasCookies() bool {
	if t.CookiesFile == "" {
		return false
	}
	st, err := os.Stat(t.CookiesFile)
	return err == nil && !st.IsDir() && st.Size() > 0
}

// PrepareYtDlpArgs constructs safe baseline arguments with an isolated per-invocation copy
// of the cookie jar so concurrent yt-dlp processes never overwrite or corrupt the master file.
func (t *Tooling) PrepareYtDlpArgs() (args []string, cleanup func()) {
	cleanup = func() {}
	args = []string{
		"--ignore-config",
		"--no-warnings",
		"--no-color",
		"--socket-timeout", "15",
	}

	if t.JSRuntimeArg != "" {
		args = append(args, "--js-runtimes", t.JSRuntimeArg)
	}
	if t.FFmpegPath != "" {
		args = append(args, "--ffmpeg-location", filepath.Dir(t.FFmpegPath))
	}
	if t.PotURL != "" {
		args = append(args,
			"-4",
			"--extractor-args", "youtubepot-bgutilhttp:base_url="+t.PotURL,
			"--extractor-args", "youtube:player-client=default,mweb",
		)
	}
	if t.HasCookies() {
		if raw, err := os.ReadFile(t.CookiesFile); err == nil && len(raw) > 0 {
			if tmp, err := os.CreateTemp("", "stash-cj-*.txt"); err == nil {
				tmpPath := tmp.Name()
				_, writeErr := tmp.Write(raw)
				closeErr := tmp.Close()
				if writeErr == nil && closeErr == nil {
					_ = os.Chmod(tmpPath, 0o600)
					args = append(args, "--cookies", tmpPath)
					cleanup = func() { _ = os.Remove(tmpPath) }
				} else {
					_ = os.Remove(tmpPath)
				}
			}
		}
	}
	if t.ProxyEnabled() {
		// Only attach the built-in warp sidecar if it can actually tunnel traffic; prevents 60s hangs if UDP 2408 is blocked.
		useProxy := true
		if isBuiltInWarpProxy(t.ProxyURL) && !t.CheckProxy(context.Background()) {
			useProxy = false
		}
		if useProxy {
			args = append(args, "--proxy", t.ProxyURL)
		}
	}

	return args, cleanup
}

// ProxyEnabled reports whether an outbound proxy URL is configured.
func (t *Tooling) ProxyEnabled() bool {
	return t.ProxyURL != "" && !strings.EqualFold(t.ProxyURL, "direct") && !strings.EqualFold(t.ProxyURL, "none")
}

// CheckProxy probes whether the configured proxy can actually reach Cloudflare (cached 30s).
func (t *Tooling) CheckProxy(ctx context.Context) bool {
	if !t.ProxyEnabled() {
		return false
	}

	t.proxyMu.Lock()
	if time.Since(t.proxyLastCheck) < 30*time.Second {
		healthy := t.proxyHealthy
		t.proxyMu.Unlock()
		return healthy
	}
	t.proxyMu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use curl (bundled in runtime image) because stdlib net/http lacks native socks5h:// transport without x/net.
	healthy := false
	if curlBin, err := exec.LookPath("curl"); err == nil {
		cmd := NewGroupCommand(probeCtx, curlBin, "-fsS", "-x", t.ProxyURL, "--max-time", "4", "https://www.cloudflare.com/cdn-cgi/trace")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Run(); err == nil && bytes.Contains(out.Bytes(), []byte("ip=")) {
			healthy = true
		}
	}

	t.proxyMu.Lock()
	t.proxyHealthy = healthy
	t.proxyLastCheck = time.Now()
	t.proxyMu.Unlock()
	return healthy
}

func isBuiltInWarpProxy(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), "warp")
}

// CheckPotProvider verifies whether the configured PO-token sidecar responds over HTTP (cached for 15s).
func (t *Tooling) CheckPotProvider(ctx context.Context) bool {
	if t.PotURL == "" {
		return false
	}

	t.potMu.Lock()
	if time.Since(t.potLastCheck) < 15*time.Second {
		healthy := t.potHealthy
		t.potMu.Unlock()
		return healthy
	}
	t.potMu.Unlock()

	reqCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	pingURL := strings.TrimRight(t.PotURL, "/") + "/ping"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, pingURL, nil)
	healthy := false
	if err == nil {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			healthy = resp.StatusCode >= 200 && resp.StatusCode < 500
		}
	}

	t.potMu.Lock()
	t.potHealthy = healthy
	t.potLastCheck = time.Now()
	t.potMu.Unlock()
	return healthy
}

// NewGroupCommand prepares a subprocess in its own POSIX process group so cancellation kills all children cleanly.
func NewGroupCommand(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 3 * time.Second
	return cmd
}

func resolveBinary(override, name string, preferUserLocal bool) string {
	if override != "" {
		if p, err := exec.LookPath(override); err == nil {
			return p
		}
	}
	if preferUserLocal {
		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, ".local", "bin", name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return candidate
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func resolveJSRuntime(override string) (name string, arg string) {
	if override != "" {
		return override, override
	}
	if denoPath, err := exec.LookPath("deno"); err == nil {
		return "deno", "deno:" + denoPath
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".deno", "bin", "deno")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return "deno", "deno:" + candidate
		}
	}
	if nodePath, err := exec.LookPath("node"); err == nil {
		return "node", "node:" + nodePath
	}
	return "none", ""
}

func runSimple(ctx context.Context, bin string, args ...string) (string, error) {
	cmd := NewGroupCommand(ctx, bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}
