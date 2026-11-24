package engine

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"stash-backend/internal/security"
)

// ZipEntry describes a verified local file to stream into a zero-compression ZIP archive.
type ZipEntry struct {
	BaseDir     string
	FilePath    string
	ArchiveName string
}

var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// StreamStoredZip writes media files directly into dst using zip.Store (zero CPU compression, zero temp zip disk usage).
func StreamStoredZip(dst io.Writer, entries []ZipEntry) error {
	zw := zip.NewWriter(dst)
	usedNames := make(map[string]int, len(entries))

	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)
	buf := *bufPtr

	for _, entry := range entries {
		safePath, err := security.EnsurePathWithinDir(entry.BaseDir, entry.FilePath)
		if err != nil {
			return fmt.Errorf("unsafe archive source path: %w", err)
		}

		info, err := os.Stat(safePath)
		if err != nil || info.IsDir() {
			continue
		}

		uniqueName := deduplicateName(sanitizeZipEntryName(entry.ArchiveName), usedNames)

		hdr := &zip.FileHeader{
			Name:     uniqueName,
			Method:   zip.Store, // MP4/WebM/M4A/Opus/MP3 are already compressed; Store streams at full I/O speed.
			Modified: time.Now(),
		}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			_ = zw.Close()
			return err
		}

		f, err := os.Open(safePath)
		if err != nil {
			_ = zw.Close()
			return err
		}

		_, copyErr := io.CopyBuffer(w, f, buf)
		closeErr := f.Close()
		if copyErr != nil {
			_ = zw.Close()
			return copyErr
		}
		if closeErr != nil {
			_ = zw.Close()
			return closeErr
		}
	}

	return zw.Close()
}

func sanitizeZipEntryName(name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	cleanStem := security.SanitizeFilename(stem, 110)
	cleanExt := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, ext)
	return cleanStem + cleanExt
}

func deduplicateName(name string, counts map[string]int) string {
	lower := strings.ToLower(name)
	n := counts[lower]
	counts[lower] = n + 1
	if n == 0 {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s (%d)%s", stem, n+1, ext)
}
