package rds

import (
	"context"
	"fmt"
	"time"

	"github.com/convox/convox/provider/aws/provisioner"
)

// Database instance status: http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Overview.DBInstance.Status.html
var DbInstanceCreatePendingStates = []string{
	"backing-up",
	"configuring-enhanced-monitoring",
	"configuring-iam-database-auth",
	"configuring-log-exports",
	"creating",
	"maintenance",
	"modifying",
	"rebooting",
	"renaming",
	"resetting-master-credentials",
	"starting",
	"stopping",
	"upgrading",
}

var DbInstanceDeletePendingStates = []string{
	"available",
	"backing-up",
	"configuring-enhanced-monitoring",
	"configuring-log-exports",
	"creating",
	"deleting",
	"incompatible-parameters",
	"modifying",
	"starting",
	"stopping",
	"storage-full",
	"storage-optimization",
}

var DbInstanceUpdatePendingStates = []string{
	"backing-up",
	"configuring-enhanced-monitoring",
	"configuring-iam-database-auth",
	"configuring-log-exports",
	"creating",
	"maintenance",
	"modifying",
	"moving-to-vpc",
	"rebooting",
	"renaming",
	"resetting-master-credentials",
	"starting",
	"stopping",
	"storage-full",
	"upgrading",
}

var DbInstanceAvailableStates = []string{"available", "storage-optimization"}

type DBStatusWaiterConf struct {
	Pending []string
	Target  []string
	Timeout time.Duration
	Delay   time.Duration
}

func (p *Provisioner) waitUntilTargetDBIsAvailableAfterInstall(dbIdentifier string) error {
	return p.waitUntilTargetDBStatus(dbIdentifier, &DBStatusWaiterConf{
		Pending: DbInstanceCreatePendingStates,
		Target:  DbInstanceAvailableStates,
		Timeout: 30 * time.Minute,
		Delay:   30 * time.Second,
	})
}

func (p *Provisioner) waitUntilTargetDBIsAvailableAfterUpdate(dbIdentifier string) error {
	return p.waitUntilTargetDBStatus(dbIdentifier, &DBStatusWaiterConf{
		Pending: DbInstanceUpdatePendingStates,
		Target:  DbInstanceAvailableStates,
		Timeout: 30 * time.Minute,
		Delay:   30 * time.Second,
	})
}

func (p *Provisioner) waitUntilTargetDBIsDeleted(dbIdentifier string) error {
	return p.waitUntilTargetDBStatus(dbIdentifier, &DBStatusWaiterConf{
		Pending: DbInstanceDeletePendingStates,
		Target:  []string{},
		Timeout: 30 * time.Minute,
		Delay:   30 * time.Second,
	})
}

func (p *Provisioner) waitUntilTargetDBIsAvailableAfterInstallOrUpdate(dbIdentifier string) error {
	return p.waitUntilTargetDBStatus(dbIdentifier, &DBStatusWaiterConf{
		Pending: append(DbInstanceCreatePendingStates, DbInstanceUpdatePendingStates...),
		Target:  []string{"available", "storage-optimization"},
		Timeout: 30 * time.Minute,
		Delay:   30 * time.Second,
	})
}

func (p *Provisioner) waitUntilTargetDBStatus(dbIdentifier string, conf *DBStatusWaiterConf) error {
	ctx, cancel := context.WithTimeout(context.Background(), conf.Timeout)
	defer cancel()

	ticker := time.NewTicker(conf.Delay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout reached: %s", ctx.Err())
		case <-ticker.C:
			status, err := p.getDbStatus(dbIdentifier)
			if err != nil {
				return err
			}

			if provisioner.TargetExistsInStringArray(conf.Target, status) {
				return nil
			}

			if provisioner.TargetExistsInStringArray(conf.Pending, status) {
				p.logger.Logf("DB instances still in %s state", status)
			}
		}
	}
}

func (p *Provisioner) getDbStatus(dbIdentifier string) (string, error) {
	db, err := p.GetDBInstance(dbIdentifier)
	if err != nil {
		return "", err
	}
	status := ""
	if db != nil {
		status = *db.DBInstanceStatus
	}
	return status, nil
}
