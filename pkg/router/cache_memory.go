package router

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/crypto/acme/autocert"
)

// todo: use sync.RWMutex
type CacheMemory struct {
	cache sync.Map
}

func NewCacheMemory() *CacheMemory {
	fmt.Printf("ns=cache.memory at=new\n")

	return &CacheMemory{
		cache: sync.Map{},
	}
}

func (c *CacheMemory) Delete(ctx context.Context, key string) error {
	fmt.Printf("ns=cache.memory at=delete key=%s\n", key)

	c.cache.Delete(key)

	return nil
}

func (c *CacheMemory) Get(ctx context.Context, key string) ([]byte, error) {
	fmt.Printf("ns=cache.memory at=get key=%s\n", key)

	v, ok := c.cache.Load(key)
	if !ok {
		return nil, autocert.ErrCacheMiss
	}

	data, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid data stored in cache")
	}

	return data, nil
}

func (c *CacheMemory) Put(ctx context.Context, key string, data []byte) error {
	fmt.Printf("ns=cache.memory at=put key=%s\n", key)

	c.cache.Store(key, data)

	return nil
}
