package router

import (
	"context"
	"fmt"

	"github.com/go-redis/redis"
	"golang.org/x/crypto/acme/autocert"
)

type CacheRedis struct {
	redis *redis.Client
}

func NewCacheRedis(addr string) (*CacheRedis, error) {
	fmt.Printf("ns=cache.redis at=new addr=%s\n", addr)

	rc := redis.NewClient(&redis.Options{Addr: addr})

	if _, err := rc.Ping().Result(); err != nil {
		return nil, err
	}

	r := &CacheRedis{
		redis: rc,
	}

	return r, nil
}

func (c *CacheRedis) Delete(ctx context.Context, key string) error {
	fmt.Printf("ns=cache.redis at=delete key=%s\n", key)

	if _, err := c.redis.Del(fmt.Sprintf("cache.%s", key)).Result(); err != nil {
		return err
	}

	return nil
}

func (c *CacheRedis) Get(ctx context.Context, key string) ([]byte, error) {
	fmt.Printf("ns=cache.redis at=get key=%s\n", key)

	v, err := c.redis.Get(fmt.Sprintf("cache.%s", key)).Result()
	if err == redis.Nil {
		return nil, autocert.ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}

	return []byte(v), nil
}

func (c *CacheRedis) Put(ctx context.Context, key string, data []byte) error {
	fmt.Printf("ns=cache.redis at=put key=%s\n", key)

	if _, err := c.redis.Set(fmt.Sprintf("cache.%s", key), data, 0).Result(); err != nil {
		return err
	}

	return nil
}
