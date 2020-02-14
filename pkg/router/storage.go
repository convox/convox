package router

import "time"

type Storage interface {
	HostList() ([]string, error)
	RequestBegin(target string) error
	RequestEnd(target string) error
	Stale(cutoff time.Time) ([]string, error)
	TargetAdd(host, target string, idles bool) error // should be idempotent
	TargetList(host string) ([]string, error)
	TargetRemove(host, target string) error
}
