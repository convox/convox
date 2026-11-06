package elasticache

import (
	"context"
	"fmt"
	"time"

	"github.com/convox/convox/provider/aws/provisioner"
)

const (
	ReplicationGroupStatusCreating     = "creating"
	ReplicationGroupStatusAvailable    = "available"
	ReplicationGroupStatusModifying    = "modifying"
	ReplicationGroupStatusDeleting     = "deleting"
	ReplicationGroupStatusCreateFailed = "create-failed"
	ReplicationGroupStatusSnapshotting = "snapshotting"

	CacheClusterStatusAvailable             = "available"
	CacheClusterStatusCreating              = "creating"
	CacheClusterStatusDeleted               = "deleted"
	CacheClusterStatusDeleting              = "deleting"
	CacheClusterStatusIncompatibleNetwork   = "incompatible-network"
	CacheClusterStatusModifying             = "modifying"
	CacheClusterStatusRebootingClusterNodes = "rebooting cluster nodes"
	CacheClusterStatusRestoreFailed         = "restore-failed"
	CacheClusterStatusSnapshotting          = "snapshotting"
)

type StatusWaiterConf struct {
	Pending []string
	Target  []string
	Timeout time.Duration
	Delay   time.Duration
}

func (p *Provisioner) waitUntilTargetReplicationGroupIsAvailable(identifier string) error {
	return p.waitUntilTargetReplicationGroupStatus(identifier, &StatusWaiterConf{
		Pending: []string{
			ReplicationGroupStatusCreating,
			ReplicationGroupStatusModifying,
			ReplicationGroupStatusSnapshotting,
		},
		Target: []string{
			ReplicationGroupStatusAvailable,
		},
		Timeout: 30 * time.Minute,
		Delay:   30 * time.Second,
	}, p.getReplicationGroupStatus)
}

func (p *Provisioner) waitUntilTargetCacheClusterIsAvailable(identifier string) error {
	return p.waitUntilTargetReplicationGroupStatus(identifier, &StatusWaiterConf{
		Pending: []string{
			CacheClusterStatusCreating,
			CacheClusterStatusModifying,
			CacheClusterStatusSnapshotting,
			CacheClusterStatusRebootingClusterNodes,
		},
		Target: []string{
			CacheClusterStatusAvailable,
		},
		Timeout: 30 * time.Minute,
		Delay:   30 * time.Second,
	}, p.getCacheClusterStatus)
}

func (p *Provisioner) waitUntilTargetReplicationGroupStatus(id string, conf *StatusWaiterConf, statusFn func(string) (string, error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), conf.Timeout)
	defer cancel()

	ticker := time.NewTicker(conf.Delay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout reached: %s", ctx.Err())
		case <-ticker.C:
			status, err := statusFn(id)
			if err != nil {
				return err
			}

			if provisioner.TargetExistsInStringArray(conf.Target, status) {
				return nil
			}
			if provisioner.TargetExistsInStringArray(conf.Pending, status) {
				p.logger.Logf("elastic cache still in %s state", status)
			}
		}
	}
}

func (p *Provisioner) getReplicationGroupStatus(replicationGroupId string) (string, error) {
	rp, err := p.GetReplicationGroup(replicationGroupId)
	if err != nil {
		return "", err
	}
	status := ""
	if rp != nil {
		status = *rp.Status
	}
	return status, nil
}

func (p *Provisioner) getCacheClusterStatus(clusterId string) (string, error) {
	resp, err := p.GetCacheCluster(clusterId)
	if err != nil {
		return "", err
	}
	status := ""
	if resp != nil {
		status = *resp.CacheClusterStatus
	}
	return status, nil
}
