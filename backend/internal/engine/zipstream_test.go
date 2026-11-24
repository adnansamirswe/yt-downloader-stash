package engine

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamStoredZip(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "track1.m4a")
	f2 := filepath.Join(dir, "track2.m4a")
	if err := os.WriteFile(f1, []byte("mock-audio-payload-one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("mock-audio-payload-two"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := StreamStoredZip(&buf, []ZipEntry{
		{BaseDir: dir, FilePath: f1, ArchiveName: "01 - Same Name.m4a"},
		{BaseDir: dir, FilePath: f2, ArchiveName: "01 - Same Name.m4a"},
	})
	if err != nil {
		t.Fatalf("StreamStoredZip failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader failed: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(zr.File))
	}
	if zr.File[0].Method != zip.Store || zr.File[1].Method != zip.Store {
		t.Fatalf("expected Store method, got %v / %v", zr.File[0].Method, zr.File[1].Method)
	}
	if zr.File[0].Name == zr.File[1].Name {
		t.Fatalf("expected deduplicated names, both were %q", zr.File[0].Name)
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(content) != "mock-audio-payload-one" {
		t.Fatalf("unexpected zip entry content: %q", string(content))
	}
}
