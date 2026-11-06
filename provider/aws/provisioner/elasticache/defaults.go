package elasticache

import (
	"fmt"
	"strings"
)

func (p *Provisioner) ApplyReplicationGroupInstallDefaults(options map[string]string) error {
	if _, has := options[ParamPort]; !has {
		options[ParamPort] = DefaultCachePort(options[ParamEngine])
	}

	if strings.EqualFold(options[ParamTransitEncryptionEnabled], "true") {
		if _, has := options[ParamAuthToken]; !has {
			return fmt.Errorf("when transit encryption is enabled, authToken/password param is required")
		}
	}
	return nil
}

func (p *Provisioner) ApplyCacheClusterInstallDefaults(options map[string]string) error {
	if _, has := options[ParamPort]; !has {
		options[ParamPort] = DefaultCachePort(options[ParamEngine])
	}

	return nil
}

func DefaultCachePort(engine string) string {
	switch engine {
	case "redis":
		return "6379"
	case "memcached":
		return "11211"
	default:
		return "8080"
	}
}
