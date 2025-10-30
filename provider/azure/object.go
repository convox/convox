package azure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Azure/azure-storage-file-go/azfile"
	"github.com/convox/convox/pkg/structs"
)

const maxRangeSize = 4 * 1024 * 1024 // 4 MiB (Azure Files max per-range for the service version in your error)

func (p *Provider) ObjectDelete(app, key string) error {
	ctx := p.Context()

	if _, err := p.storageFile(p.objectKey(app, key)).Delete(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Provider) ObjectExists(app, key string) (bool, error) {
	if _, err := p.storageFile(p.objectKey(app, key)).GetProperties(p.Context()); err != nil {
		if azerr, ok := err.(azfile.StorageError); ok && azerr.ServiceCode() == "ResourceNotFound" {
			return false, nil
		}

		return false, err
	}

	return false, nil
}

func (p *Provider) ObjectFetch(app, key string) (io.ReadCloser, error) {
	ctx := p.Context()

	res, err := p.storageFile(p.objectKey(app, key)).Download(ctx, 0, azfile.CountToEnd, false)
	if err != nil {
		if azerr, ok := err.(azfile.StorageError); ok && azerr.ServiceCode() == "ResourceNotFound" {
			return nil, fmt.Errorf("no such key")
		}

		return nil, err
	}

	r := res.Body(azfile.RetryReaderOptions{})

	return r, nil
}

func (p *Provider) ObjectList(app, prefix string) ([]string, error) {
	ctx := p.Context()

	dir := p.storageDirectory.NewDirectoryURL(p.objectKey(app, prefix))

	fs := []string{}

	for marker := (azfile.Marker{}); marker.NotDone(); {
		res, err := dir.ListFilesAndDirectoriesSegment(ctx, marker, azfile.ListFilesAndDirectoriesOptions{})
		if err != nil {
			if azerr, ok := err.(azfile.StorageError); ok && azerr.ServiceCode() == "ResourceNotFound" {
				return []string{}, nil
			}

			return nil, err
		}

		marker = res.NextMarker

		for _, file := range res.FileItems {
			fs = append(fs, file.Name)
		}
	}

	return fs, nil
}

func (p *Provider) ObjectStore(app, key string, r io.Reader, opts structs.ObjectStoreOptions) (*structs.Object, error) {
	ctx := p.Context()

	if key == "" {
		k, err := generateTempKey()
		if err != nil {
			return nil, err
		}
		key = k
	}

	name := p.objectKey(app, key)

	if err := p.storageMkdir(name); err != nil {
		return nil, err
	}

	fw, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	defer fw.Close()
	defer os.Remove(fw.Name())

	if _, err := io.Copy(fw, r); err != nil {
		return nil, err
	}

	if err := fw.Close(); err != nil {
		return nil, err
	}

	file := p.storageFile(name)

	if err := uploadFileInRanges(ctx, file, fw.Name()); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("object://%s/%s", app, key)

	o := &structs.Object{Url: url}

	return o, nil
}

func (p *Provider) objectKey(app, key string) string {
	return fmt.Sprintf("%s/%s", app, strings.TrimPrefix(key, "/"))
}

func (p *Provider) storageFile(key string) azfile.FileURL {
	return p.storageDirectory.NewFileURL(key)
}

func (p *Provider) storageMkdir(file string) error {
	ctx := p.Context()

	parts := strings.Split(file, "/")
	if len(parts) < 2 {
		return nil
	}

	dir := *p.storageDirectory

	for _, name := range parts[0 : len(parts)-1] {
		dir = dir.NewDirectoryURL(name)

		if _, err := dir.Create(ctx, azfile.Metadata{}); err != nil {
			if azerr, ok := err.(azfile.StorageError); ok {
				if azerr.ServiceCode() == "ResourceAlreadyExists" {
					continue
				}
				if azerr.ServiceCode() == "ResourceTypeMismatch" {
					return fmt.Errorf("unable to create directory")
				}
			}

			return err
		}
	}

	return nil
}

func generateTempKey() (string, error) {
	data := make([]byte, 1024)

	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return fmt.Sprintf("tmp/%s", hex.EncodeToString(hash[:])[0:30]), nil
}

// UploadFileInRanges uploads a local file to the given azfile.FileURL by splitting it into <=4MiB ranges.
// - fileURL must point at the destination file on the share (FileURL).
// - localPath is the local file to upload.
func uploadFileInRanges(ctx context.Context, fileURL azfile.FileURL, localPath string) error {
	// Open local file
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}
	fileSize := stat.Size()

	if _, err := fileURL.Create(ctx, fileSize, azfile.FileHTTPHeaders{}, azfile.Metadata{}); err != nil {
		return fmt.Errorf("create remote file (size %d): %w", fileSize, err)
	}

	var offset int64 = 0
	buf := make([]byte, maxRangeSize)

	for offset < fileSize {
		remaining := fileSize - offset
		chunkSize := int64(len(buf))
		if remaining < chunkSize {
			chunkSize = remaining
		}

		// Read exactly chunkSize bytes (or less at EOF)
		n, err := io.ReadFull(f, buf[:chunkSize])
		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				// partial final read is fine
			} else {
				return fmt.Errorf("read local file at offset %d: %w", offset, err)
			}
		}

		r := bytes.NewReader(buf[:n])

		if _, err := fileURL.UploadRange(ctx, offset, r, nil); err != nil {
			return fmt.Errorf("upload range offset=%d len=%d: %w", offset, n, err)
		}

		offset += int64(n)
	}

	return nil
}
