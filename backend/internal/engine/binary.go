package engine

import (
	"bytes"
	"context"
	"net/http"
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
	ProxyURL      string

	potMu        sync.Mutex
	potLastCheck time.Time
	potHealthy   bool
}

// DetectTooling resolves yt-dlp, ffmpeg, and JS runtime binaries once at startup.
func DetectTooling(ctx context.Context, cfg config.Config) *Tooling {
	t := &Tooling{
		PotURL:      cfg.PotProviderURL,
		CookiesFile: cfg.CookiesFile,
		ProxyURL:    cfg.ProxyURL,
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

// BaseYtDlpArgs constructs safe baseline arguments shared by inspect and download commands.
func (t *Tooling) BaseYtDlpArgs() []string {
	args := []string{
		"--ignore-config",
		"--no-warnings",
		"--no-color",
	}

	if t.JSRuntimeArg != "" {
		args = append(args, "--js-runtimes", t.JSRuntimeArg)
	}
	if t.FFmpegPath != "" {
		args = append(args, "--ffmpeg-location", filepath.Dir(t.FFmpegPath))
	}
	if t.PotURL != "" {
		args = append(args, "--extractor-args", "youtubepot-bgutilhttp:base_url="+t.PotURL)
	}
	if t.CookiesFile != "" {
		if st, err := os.Stat(t.CookiesFile); err == nil && !st.IsDir() {
			args = append(args, "--cookies", t.CookiesFile)
		}
	}
	if t.ProxyURL != "" {
		args = append(args, "--proxy", t.ProxyURL)
	}

	return args
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
