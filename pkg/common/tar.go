package common

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/archive"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
)

var (
	maxTarEntrySize int64 = 10 * 1024 * 1024 * 1024 // 10GB per entry
	maxTarTotalSize int64 = 50 * 1024 * 1024 * 1024  // 50GB cumulative
)

func Archive(file string) (io.Reader, error) {
	opts := &archive.TarOptions{
		IncludeFiles: []string{file},
	}

	r, err := archive.TarWithOptions("/", opts)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func RebaseArchive(r io.Reader, src, dst string) (io.Reader, error) {
	tr := tar.NewReader(r)

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	var totalSize int64

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if !strings.HasPrefix(h.Name, "/") {
			h.Name = fmt.Sprintf("/%s", h.Name)
		}

		// the second check - strings.HasSuffix(h.Name, "/") is for checking if the src provided is a single file, if it is then it should not be skipped --
		if !strings.HasPrefix(h.Name, src) && strings.HasSuffix(h.Name, "/") {
			continue
		}

		h.Name = filepath.Join(dst, strings.TrimPrefix(h.Name, src))

		tw.WriteHeader(h)

		n, err := io.Copy(tw, io.LimitReader(tr, maxTarEntrySize))
		if err != nil {
			return nil, err
		}
		if n >= maxTarEntrySize {
			return nil, fmt.Errorf("tar entry %q exceeds maximum entry size of %d bytes", h.Name, maxTarEntrySize)
		}
		totalSize += n
		if totalSize > maxTarTotalSize {
			return nil, fmt.Errorf("archive exceeds maximum total size of %d bytes", maxTarTotalSize)
		}
	}

	tw.Close()

	return &buf, nil
}

func Tarball(dir string) ([]byte, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	sym, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(filepath.Join(sym, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	excludes, err := dockerignore.ReadAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	defer os.Chdir(cwd)

	if err := os.Chdir(sym); err != nil {
		return nil, err
	}

	opts := &archive.TarOptions{
		Compression:     archive.Gzip,
		ExcludePatterns: excludes,
		IncludeFiles:    []string{"."},
	}

	r, err := archive.TarWithOptions(sym, opts)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(io.LimitReader(r, maxTarTotalSize))
}

func Unarchive(r io.Reader, target string) error {
	tr := tar.NewReader(r)

	cleanTarget := filepath.Clean(target) + string(os.PathSeparator)
	var totalSize int64

	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		cleanPath := filepath.Clean(filepath.Join(target, h.Name))
		if !strings.HasPrefix(cleanPath, cleanTarget) && cleanPath != filepath.Clean(target) {
			return fmt.Errorf("illegal file path in archive: %s", h.Name)
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanPath, os.FileMode(h.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanPath), 0700); err != nil {
				return err
			}

			fd, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY, os.FileMode(h.Mode))
			if err != nil {
				return err
			}

			n, err := io.Copy(fd, io.LimitReader(tr, maxTarEntrySize))
			fd.Close()
			if err != nil {
				return err
			}
			if n >= maxTarEntrySize {
				return fmt.Errorf("tar entry %q exceeds maximum entry size of %d bytes", h.Name, maxTarEntrySize)
			}
			totalSize += n
			if totalSize > maxTarTotalSize {
				return fmt.Errorf("archive exceeds maximum total size of %d bytes", maxTarTotalSize)
			}
		}
	}
}
