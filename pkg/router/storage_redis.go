package router

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

const ()

type StorageRedis struct {
	redis *redis.Client
}

func NewStorageRedis(addr, password string, secure bool) (*StorageRedis, error) {
	fmt.Printf("ns=storage.redis at=new addr=%s\n", addr)

	opts := &redis.Options{
		Addr:     addr,
		Password: password,
	}

	if secure {
		opts.TLSConfig = &tls.Config{}
	}

	rc := redis.NewClient(opts)

	if _, err := rc.Ping().Result(); err != nil {
		return nil, err
	}

	r := &StorageRedis{
		redis: rc,
	}

	return r, nil
}

func (s *StorageRedis) HostList() ([]string, error) {
	// fmt.Printf("ns=storage.redis at=host.list\n")

	hs, err := s.redis.SMembers("router/hosts").Result()
	if err != nil {
		return nil, err
	}

	return hs, nil
}

func (s *StorageRedis) IdleGet(target string) (bool, error) {
	fmt.Printf("ns=storage.redis at=idle.get target=%q\n", target)

	idle, err := s.redis.Get(fmt.Sprintf("router/idle/%s", target)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return idle == "true", nil
}

func (s *StorageRedis) IdleSet(target string, idle bool) error {
	fmt.Printf("ns=storage.redis at=idle.get target=%q idle=%t\n", target, idle)

	if _, err := s.redis.Set(fmt.Sprintf("router/idle/%s", target), fmt.Sprintf("%t", idle), 0).Result(); err != nil {
		return err
	}

	return nil
}

func (s *StorageRedis) RequestBegin(target string) error {
	fmt.Printf("ns=storage.redis at=request.begin target=%q\n", target)

	if _, err := s.redis.Set(fmt.Sprintf("router/activity/%s", target), time.Now().UTC(), 0).Result(); err != nil {
		return err
	}

	if _, err := s.redis.IncrBy(fmt.Sprintf("router/connections/%s", target), 1).Result(); err != nil {
		return err
	}

	return nil
}

func (s *StorageRedis) RequestEnd(target string) error {
	fmt.Printf("ns=storage.redis at=request.end target=%q\n", target)

	if _, err := s.redis.IncrBy(fmt.Sprintf("router/connections/%s", target), -1).Result(); err != nil {
		return err
	}

	return nil
}

// TODO implement
func (s *StorageRedis) Stale(cutoff time.Time) ([]string, error) {
	fmt.Printf("ns=storage.redis at=stale cutoff=%s\n", cutoff)

	return []string{}, nil
}

func (s *StorageRedis) TargetAdd(host, target string, idles bool) error {
	fmt.Printf("ns=storage.redis at=target.add host=%q target=%q idles=%t\n", host, target, idles)

	if _, err := s.redis.SAdd("router/hosts", host).Result(); err != nil {
		return err
	}

	if _, err := s.redis.LPush(fmt.Sprintf("router/targets/%s", host), target).Result(); err != nil {
		return err
	}

	return nil
}

func (s *StorageRedis) TargetList(host string) ([]string, error) {
	// fmt.Printf("ns=storage.redis at=target.list\n")

	ts, err := s.redis.LRange(fmt.Sprintf("router/targets/%s", host), 0, -1).Result()
	if err == redis.Nil {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func (s *StorageRedis) TargetRemove(host, target string) error {
	fmt.Printf("ns=storage.redis at=target.remove host=%q target=%q\n", host, target)

	if _, err := s.redis.LRem(fmt.Sprintf("router/targets/%s", host), 1, target).Result(); err != nil {
		return err
	}

	len, err := s.redis.LLen(fmt.Sprintf("router/targets/%s", host)).Result()
	if err != nil {
		return err
	}
	
	if len == 0 {
		if _, err := s.redis.SRem("router/hosts", host).Result(); err != nil {
			return err
		}
	}

	return nil
}
