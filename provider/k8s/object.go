package k8s

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
)

func (p *Provider) ObjectDelete(app, key string) error {
	err := os.Remove(p.objectFilename(app, key))
	if os.IsNotExist(err) {
		return errors.WithStack(fmt.Errorf("object not found: %s", key))
	}
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) ObjectExists(app, key string) (bool, error) {
	_, err := os.Stat(p.objectFilename(app, key))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}

func (p *Provider) ObjectFetch(app, key string) (io.ReadCloser, error) {
	fd, err := os.Open(p.objectFilename(app, key))
	if os.IsNotExist(err) {
		return nil, errors.WithStack(fmt.Errorf("object not found: %s", key))
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fd, nil
}

func (p *Provider) ObjectList(app, prefix string) ([]string, error) {
	return nil, errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) ObjectStore(app, key string, r io.Reader, opts structs.ObjectStoreOptions) (*structs.Object, error) {
	if key == "" {
		k, err := generateTempKey()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		key = k
	}

	fn := p.objectFilename(app, key)

	if err := os.MkdirAll(filepath.Dir(fn), 0700); err != nil {
		return nil, errors.WithStack(err)
	}

	fd, err := os.OpenFile(fn, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := io.Copy(fd, r); err != nil {
		return nil, errors.WithStack(err)
	}

	o := &structs.Object{
		Url: fmt.Sprintf("object://%s/%s", app, key),
	}

	return o, nil
}

func (p *Provider) objectFilename(app, key string) string {
	return filepath.Join(p.Storage, "objects", app, key)
}

func generateTempKey() (string, error) {
	data := make([]byte, 1024)

	if _, err := rand.Read(data); err != nil {
		return "", errors.WithStack(err)
	}

	hash := sha256.Sum256(data)

	return fmt.Sprintf("tmp/%s", hex.EncodeToString(hash[:])[0:30]), nil
}
