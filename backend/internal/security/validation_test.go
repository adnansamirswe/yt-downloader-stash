package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateYouTubeURL(t *testing.T) {
	validCases := []struct {
		name     string
		url      string
		wantKind string
		wantVid  string
		wantList string
	}{
		{
			name:     "standard watch video",
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantKind: "video",
			wantVid:  "dQw4w9WgXcQ",
		},
		{
			name:     "youtu.be short link",
			url:      "https://youtu.be/dQw4w9WgXcQ?t=42",
			wantKind: "video",
			wantVid:  "dQw4w9WgXcQ",
		},
		{
			name:     "shorts link",
			url:      "https://youtube.com/shorts/dQw4w9WgXcQ",
			wantKind: "video",
			wantVid:  "dQw4w9WgXcQ",
		},
		{
			name:     "pure playlist link",
			url:      "https://www.youtube.com/playlist?list=PLMC9KNkIncKtPzgY-5rmhvj7fax8fdxoj",
			wantKind: "playlist",
			wantList: "PLMC9KNkIncKtPzgY-5rmhvj7fax8fdxoj",
		},
		{
			name:     "hybrid watch plus playlist",
			url:      "https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=PLMC9KNkIncKtPzgY-5rmhvj7fax8fdxoj",
			wantKind: "hybrid",
			wantVid:  "dQw4w9WgXcQ",
			wantList: "PLMC9KNkIncKtPzgY-5rmhvj7fax8fdxoj",
		},
		{
			name:     "youtube mix RD playlist derives seed watch url",
			url:      "https://youtube.com/playlist?list=RDCVtHZ5zhxIk&playnext=1&si=EzYfkbUhoYiNQcx6",
			wantKind: "playlist",
			wantVid:  "",
			wantList: "RDCVtHZ5zhxIk",
		},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateYouTubeURL(tc.url)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.url, err)
			}
			if got.Kind != tc.wantKind || got.VideoID != tc.wantVid || got.PlaylistID != tc.wantList {
				t.Fatalf("got (%s, %s, %s), want (%s, %s, %s)",
					got.Kind, got.VideoID, got.PlaylistID, tc.wantKind, tc.wantVid, tc.wantList)
			}
			if tc.wantList == "RDCVtHZ5zhxIk" && got.PlaylistURL != "https://www.youtube.com/watch?v=CVtHZ5zhxIk&list=RDCVtHZ5zhxIk" {
				t.Fatalf("expected seed watch URL for RD mix, got %q", got.PlaylistURL)
			}
		})
	}

	invalidCases := []string{
		"",
		"file:///etc/passwd",
		"http://127.0.0.1:8080/watch?v=dQw4w9WgXcQ",
		"http://169.254.169.254/latest/meta-data/",
		"https://evil.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com.evil.com/watch?v=dQw4w9WgXcQ",
		"https://user:pass@www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com:8443/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=bad",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ;rm -rf /",
		"https://www.youtube.com/playlist",
		"javascript:alert(1)",
	}

	for _, raw := range invalidCases {
		if _, err := ValidateYouTubeURL(raw); err == nil {
			t.Errorf("expected error for invalid URL %q, got nil", raw)
		}
	}
}

func TestSanitizeFilenameAndPathContainment(t *testing.T) {
	got := SanitizeFilename("../../etc/passwd: <test> | song\x00name", 64)
	if strings.Contains(got, "/") || strings.Contains(got, "\\") || strings.Contains(got, "..") || strings.ContainsRune(got, 0) {
		t.Fatalf("unsanitized characters remained: %q", got)
	}

	base := t.TempDir()
	inside := filepath.Join(base, "sub", "file.mp4")
	if err := os.MkdirAll(filepath.Dir(inside), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsurePathWithinDir(base, inside); err != nil {
		t.Fatalf("expected inside path to be allowed: %v", err)
	}

	outside := filepath.Join(base, "..", "escape.mp4")
	if _, err := EnsurePathWithinDir(base, outside); err == nil {
		t.Fatalf("expected traversal path %q to be rejected", outside)
	}
}

func TestContentDispositionAttachment(t *testing.T) {
	header := ContentDispositionAttachment("01 - Café / Live: Track.m4a")
	if !strings.HasPrefix(header, `attachment; filename="`) || !strings.Contains(header, `filename*=UTF-8''`) {
		t.Fatalf("unexpected header: %s", header)
	}
}
