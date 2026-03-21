package common

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnarchive_ZipSlip(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"parent traversal", "../../../tmp/malicious.txt"},
		{"relative subdirectory traversal", "foo/../../escape.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)

			content := []byte("malicious content")
			tw.WriteHeader(&tar.Header{
				Name:     tt.filename,
				Size:     int64(len(content)),
				Mode:     0644,
				Typeflag: tar.TypeReg,
			})
			tw.Write(content)
			tw.Close()

			dir := t.TempDir()
			err := Unarchive(&buf, dir)
			if err == nil {
				t.Fatal("expected error for path traversal, got nil")
			}
			if !strings.Contains(err.Error(), "illegal file path") {
				t.Fatalf("expected 'illegal file path' error, got: %s", err)
			}

			// verify file was not written
			maliciousPath := filepath.Clean(filepath.Join(dir, tt.filename))
			if _, err := os.Stat(maliciousPath); !os.IsNotExist(err) {
				t.Fatalf("malicious file should not exist at %s", maliciousPath)
			}
		})
	}
}

func TestUnarchive_DecompressionBombLimit(t *testing.T) {
	oldEntry := maxTarEntrySize
	maxTarEntrySize = 1024
	t.Cleanup(func() { maxTarEntrySize = oldEntry })

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Create an entry with content larger than the lowered limit
	content := make([]byte, 2048)
	for i := range content {
		content[i] = 'A'
	}
	tw.WriteHeader(&tar.Header{
		Name:     "oversized.bin",
		Size:     int64(len(content)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	})
	tw.Write(content)
	tw.Close()

	dir := t.TempDir()
	err := Unarchive(&buf, dir)
	if err == nil {
		t.Fatal("expected error for oversized entry, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum entry size") {
		t.Fatalf("expected 'exceeds maximum entry size' error, got: %s", err)
	}

	// file should either not exist or be truncated (not the full 2048)
	info, statErr := os.Stat(filepath.Join(dir, "oversized.bin"))
	if statErr == nil && info.Size() >= 2048 {
		t.Fatalf("file should not be full size, got %d bytes", info.Size())
	}
}

func TestUnarchive_CumulativeSizeLimit(t *testing.T) {
	oldTotal := maxTarTotalSize
	maxTarTotalSize = 2048
	t.Cleanup(func() { maxTarTotalSize = oldTotal })

	// Keep per-entry limit higher than any single entry
	oldEntry := maxTarEntrySize
	maxTarEntrySize = 4096
	t.Cleanup(func() { maxTarEntrySize = oldEntry })

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Create 3 entries of 1024 bytes each = 3072 total, exceeding 2048 cumulative limit
	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		content := make([]byte, 1024)
		tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		})
		tw.Write(content)
	}
	tw.Close()

	dir := t.TempDir()
	err := Unarchive(&buf, dir)
	if err == nil {
		t.Fatal("expected error for cumulative size limit, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum total size") {
		t.Fatalf("expected 'exceeds maximum total size' error, got: %s", err)
	}
}

func TestRebaseArchive_SizeLimit(t *testing.T) {
	oldEntry := maxTarEntrySize
	maxTarEntrySize = 512
	t.Cleanup(func() { maxTarEntrySize = oldEntry })

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Create an entry larger than the lowered per-entry limit
	content := make([]byte, 1024)
	for i := range content {
		content[i] = 'B'
	}
	tw.WriteHeader(&tar.Header{
		Name:     "/src/big.bin",
		Size:     int64(len(content)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	})
	tw.Write(content)
	tw.Close()

	_, err := RebaseArchive(&buf, "/src", "/dst")
	if err == nil {
		t.Fatal("expected error for oversized entry in RebaseArchive, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum entry size") {
		t.Fatalf("expected 'exceeds maximum entry size' error, got: %s", err)
	}
}
