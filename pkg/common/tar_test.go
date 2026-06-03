package common

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

type tarEntry struct {
	name     string
	typeflag byte
	body     string
}

func makeTar(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0644, Size: int64(len(e.body)), Typeflag: e.typeflag}
		if e.typeflag == tar.TypeDir {
			h.Mode = 0755
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if len(e.body) > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %q: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buf.Bytes()
}

func TestUnarchiveRejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	data := makeTar(t, []tarEntry{{name: "../escape.txt", typeflag: tar.TypeReg, body: "pwned"}})

	if err := Unarchive(bytes.NewReader(data), tmp); err == nil {
		t.Fatal("expected error for traversal entry, got nil")
	}

	outside := filepath.Join(filepath.Dir(tmp), "escape.txt")
	if _, err := os.Stat(outside); err == nil {
		t.Fatalf("traversal file was written outside target: %s", outside)
	}
}

func TestRebaseArchiveRejectsEscape(t *testing.T) {
	data := makeTar(t, []tarEntry{{name: "/base/../../etc/passwd", typeflag: tar.TypeReg, body: "x"}})

	if _, err := RebaseArchive(bytes.NewReader(data), "/base", "/tmp/safezone"); err == nil {
		t.Fatal("expected error for entry escaping dst, got nil")
	}
}

func TestIsContained(t *testing.T) {
	cases := []struct {
		base, path string
		want       bool
	}{
		{"/", "/home/u/d/file", true}, // cp remote->local, target="/"
		{".", "src/main.go", true},    // build source, target="."
		{".", ".", true},
		{"/tmp/x", "/tmp/x/app.json", true}, // app import
		{"/tmp/x", "/tmp/x", true},          // root entry exact match
		{"/", "/anything", true},            // cannot escape root
		{"/tmp/x", "/etc/passwd", false},    // escape
		{".", "../escape", false},
		{"/tmp/x", "/tmp/xyz/file", false}, // sibling prefix: not contained
		{"/tmp/x", "/tmp/x/../y", false},   // resolves outside base
	}
	for _, c := range cases {
		if got := isContained(c.base, c.path); got != c.want {
			t.Errorf("isContained(%q, %q) = %v, want %v", c.base, c.path, got, c.want)
		}
	}
}

func TestSafeExtractPath(t *testing.T) {
	if _, err := SafeExtractPath("/tmp/x", "app/file"); err != nil {
		t.Fatalf("benign path rejected: %v", err)
	}
	if _, err := SafeExtractPath("/tmp/x", "../../etc/passwd"); err == nil {
		t.Fatal("expected escape to be rejected, got nil")
	}
	got, err := SafeExtractPath("/tmp/x", "web.tar")
	if err != nil || got != "/tmp/x/web.tar" {
		t.Fatalf("SafeExtractPath = (%q, %v), want (/tmp/x/web.tar, nil)", got, err)
	}
}

func TestUnarchiveBenign(t *testing.T) {
	tmp := t.TempDir()
	data := makeTar(t, []tarEntry{
		{name: "app", typeflag: tar.TypeDir},
		{name: "app/config.yml", typeflag: tar.TypeReg, body: "name: web"},
	})

	if err := Unarchive(bytes.NewReader(data), tmp); err != nil {
		t.Fatalf("benign extraction failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmp, "app", "config.yml"))
	if err != nil {
		t.Fatalf("expected app/config.yml extracted: %v", err)
	}
	if string(got) != "name: web" {
		t.Fatalf("content = %q, want %q", got, "name: web")
	}
}

func TestUnarchiveBuildDotTarget(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	data := makeTar(t, []tarEntry{
		{name: "src", typeflag: tar.TypeDir},
		{name: "src/main.go", typeflag: tar.TypeReg, body: "package main"},
	})

	if err := Unarchive(bytes.NewReader(data), "."); err != nil {
		t.Fatalf("build-style extraction to \".\" failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "src", "main.go")); err != nil {
		t.Fatalf("expected src/main.go extracted: %v", err)
	}
}

func fastTarCaps(t *testing.T, entry, total int64) {
	t.Helper()
	prevEntry, prevTotal := maxTarEntrySize, maxTarTotalSize
	t.Cleanup(func() {
		maxTarEntrySize = prevEntry
		maxTarTotalSize = prevTotal
	})
	maxTarEntrySize = entry
	maxTarTotalSize = total
}

func TestUnarchiveRejectsOversizedEntry(t *testing.T) {
	fastTarCaps(t, 8, 1000)
	tmp := t.TempDir()
	data := makeTar(t, []tarEntry{{name: "big.bin", typeflag: tar.TypeReg, body: "123456789"}}) // 9 > 8

	if err := Unarchive(bytes.NewReader(data), tmp); err == nil {
		t.Fatal("expected oversized-entry error, got nil")
	}
}

func TestUnarchiveAllowsEntryExactlyAtCap(t *testing.T) {
	fastTarCaps(t, 8, 1000)
	tmp := t.TempDir()
	data := makeTar(t, []tarEntry{{name: "exact.bin", typeflag: tar.TypeReg, body: "12345678"}}) // exactly 8

	if err := Unarchive(bytes.NewReader(data), tmp); err != nil {
		t.Fatalf("entry exactly at cap should be allowed, got %v", err)
	}
}

func TestUnarchiveRejectsOversizedTotal(t *testing.T) {
	fastTarCaps(t, 100, 10)
	tmp := t.TempDir()
	data := makeTar(t, []tarEntry{
		{name: "a.bin", typeflag: tar.TypeReg, body: "123456"}, // 6
		{name: "b.bin", typeflag: tar.TypeReg, body: "123456"}, // 6 -> total 12 > 10
	})

	if err := Unarchive(bytes.NewReader(data), tmp); err == nil {
		t.Fatal("expected cumulative-cap error, got nil")
	}
}

func TestRebaseArchiveBenign(t *testing.T) {
	data := makeTar(t, []tarEntry{{name: "/base/app/main.go", typeflag: tar.TypeReg, body: "x"}})

	rr, err := RebaseArchive(bytes.NewReader(data), "/base", "/dst")
	if err != nil {
		t.Fatalf("benign rebase failed: %v", err)
	}

	h, err := tar.NewReader(rr).Next()
	if err != nil {
		t.Fatalf("read rebased entry: %v", err)
	}
	if h.Name != "/dst/app/main.go" {
		t.Fatalf("rebased name = %q, want /dst/app/main.go", h.Name)
	}
}
