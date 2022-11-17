package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runTar() error {
	var buf bytes.Buffer

	// tmp, _ := ioutil.TempDir("", "")
	src := "./layers"
	// src := "."
	zr := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zr)

	// os.Mkdir(fmt.Sprintf("%s/svc.build", tmp), 0777)

	// walk through every file in the folder
	filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if file == "main.go" {
			return nil
		}

		if strings.HasSuffix(file, "gzip") {
			return nil
		}
		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// must provide real name
		// (see https://golang.org/src/archive/tar/common.go?#L626)
		// header.Name = filepath.ToSlash(file)
		// println(file)
		// ff := strings.Split(file, "/")
		// fname := strings.Join(ff[2:], "/")

		// println(fname)
		header.Name = filepath.ToSlash(file)

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})

	// produce tar
	if err := tw.Close(); err != nil {
		return err
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		return err
	}

	// write the .tar.gzip
	fileToWrite, err := os.OpenFile("./compress.tar.gzip", os.O_CREATE|os.O_RDWR, os.FileMode(600))
	if err != nil {
		panic(err)
	}
	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		panic(err)
	}

	return nil
}

func runTarDecompress() {
	f, _ := os.Open("./compress.tar.gzip")

	gz, _ := gzip.NewReader(f)
	tz := tar.NewReader(gz)

	for {
		header, _ := tz.Next()

		if header != nil {
			ff, _ := os.Open(header.Name)
			println(ff.Name())

		}
	}
}

func main() {
	// runTar()
	runTarDecompress()
}
