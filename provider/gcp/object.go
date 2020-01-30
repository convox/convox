package gcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/convox/convox/pkg/structs"
	"google.golang.org/api/iterator"
)

func (p *Provider) ObjectDelete(app, key string) error {
	exists, err := p.ObjectExists(app, p.objectKey(app, key))
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("object not found: %s", key)
	}

	if err := p.storage.Bucket(p.Bucket).Object(p.objectKey(app, key)).Delete(p.Context()); err != nil {
		return err
	}

	return nil
}

func (p *Provider) ObjectExists(app, key string) (bool, error) {
	_, err := p.storage.Bucket(p.Bucket).Object(p.objectKey(app, key)).Attrs(p.Context())
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// ObjectFetch fetches an Object
func (p *Provider) ObjectFetch(app, key string) (io.ReadCloser, error) {
	r, err := p.storage.Bucket(p.Bucket).Object(p.objectKey(app, key)).NewReader(p.Context())
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (p *Provider) ObjectList(app, prefix string) ([]string, error) {
	os := []string{}

	it := p.storage.Bucket(p.Bucket).Objects(p.Context(), nil)

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		os = append(os, attrs.Name)
	}

	return os, nil
}

// ObjectStore stores an Object
func (p *Provider) ObjectStore(app, key string, r io.Reader, opts structs.ObjectStoreOptions) (*structs.Object, error) {
	if key == "" {
		k, err := generateTempKey()
		if err != nil {
			return nil, err
		}
		key = k
	}

	o := p.storage.Bucket(p.Bucket).Object(p.objectKey(app, key))

	w := o.NewWriter(p.Context())

	if _, err := io.Copy(w, r); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	attrs, err := o.Attrs(p.Context())
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("object://%s/%s", app, key)

	if opts.Public != nil && *opts.Public {
		url = attrs.MediaLink
	}

	oo := &structs.Object{Url: url}

	return oo, nil
}

func (p *Provider) objectKey(app, key string) string {
	return fmt.Sprintf("%s/%s", app, strings.TrimPrefix(key, "/"))
}

func generateTempKey() (string, error) {
	data := make([]byte, 1024)

	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return fmt.Sprintf("tmp/%s", hex.EncodeToString(hash[:])[0:30]), nil
}
