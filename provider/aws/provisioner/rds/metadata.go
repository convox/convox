package rds

import (
	"github.com/convox/convox/pkg/options"
)

type MetaData struct {
	Required                    *bool   `json:"required"`
	Ignore                      *bool   `json:"ignore"`
	Immutable                   *bool   `json:"immutable"`
	UpdatesWithSomeInterruption *bool   `json:"updatesWithSomeInterruption"`
	Default                     *string `json:"default"`
}

func GetParametersMetaDataForInstall() map[string]*MetaData {
	return map[string]*MetaData{
		ParamDBInstanceIdentifier: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamDBName: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
			Default:   options.String("app"),
		},
		ParamDBParameterGroupName: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamEngine: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamEngineVersion: {
			Required:                    options.Bool(true),
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBInstanceClass: {
			Required:                    options.Bool(true),
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamStorageType: {
			Required: options.Bool(true),
			Default:  options.String("gp2"),
		},
		ParamAllocatedStorage: {
			Required: options.Bool(true),
			Default:  options.String("20"),
		},
		ParamMasterUsername: {
			Required: options.Bool(true),
			Default:  options.String("app"),
		},
		ParamMasterUserPassword: {
			Required: options.Bool(true),
		},
		ParamAllowMajorVersionUpgrade: {
			Default: options.String("true"),
		},
		ParamAutoMinorVersionUpgrade: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamBackupRetentionPeriod: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBSubnetGroupName: {
			// if not provided, convox will create it
			Immutable: options.Bool(true),
		},
		ParamDBSnapshotIdentifier: {
			Immutable: options.Bool(true),
		},
		ParamDeletionProtection: {},
		ParamIops:               {},
		ParamMultiAZ: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamPort: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamPreferredBackupWindow:      {},
		ParamPreferredMaintenanceWindow: {},
		ParamPubliclyAccessible: {
			Default: options.String("false"),
		},
		ParamStorageEncrypted: {
			Immutable: options.Bool(true),
		},
		ParamSubnetIds:         {},
		ParamVPCSecurityGroups: {},
		ParamVPC:               {},
	}

	// not allowed params:
	// - ParamSourceDBInstanceIdentifier
}

func GetParametersMetaDataForUpdate() map[string]*MetaData {
	return map[string]*MetaData{
		ParamDBInstanceIdentifier: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamDBName: {
			Immutable: options.Bool(true),
			Default:   options.String("app"),
		},
		ParamDBParameterGroupName: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamEngine: {
			Immutable: options.Bool(true),
		},
		ParamEngineVersion: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBInstanceClass: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamStorageType: {
			Default: options.String("gp2"),
		},
		ParamAllocatedStorage: {
			Default: options.String("20"),
		},
		ParamMasterUsername: {
			Immutable: options.Bool(true),
		},
		ParamMasterUserPassword: {},
		ParamAllowMajorVersionUpgrade: {
			Default: options.String("true"),
		},
		ParamAutoMinorVersionUpgrade: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamBackupRetentionPeriod: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBSubnetGroupName: {
			Immutable: options.Bool(true),
		},
		ParamDBSnapshotIdentifier: {
			Immutable: options.Bool(true),
		},
		ParamDeletionProtection: {},
		ParamIops:               {},
		ParamMultiAZ: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamPort: {
			Immutable: options.Bool(true),
		},
		ParamPreferredBackupWindow:      {},
		ParamPreferredMaintenanceWindow: {},
		ParamPubliclyAccessible: {
			Default: options.String("false"),
		},
		ParamStorageEncrypted: {
			Immutable: options.Bool(true),
		},
		ParamSubnetIds:         {},
		ParamVPCSecurityGroups: {},
		ParamVPC:               {},
		ParamApplyImmediately: {
			Default: options.String("true"),
		},
	}
}

func GetParametersMetaDataForImport() map[string]*MetaData {
	m := map[string]*MetaData{}
	for _, p := range ParametersNameList() {
		m[p] = &MetaData{}
	}

	m[ParamDBInstanceIdentifier] = &MetaData{
		Required:  options.Bool(true),
		Immutable: options.Bool(true),
	}

	m[ParamPort] = &MetaData{
		Required: options.Bool(true),
	}

	m[ParamDBName] = &MetaData{
		Required: options.Bool(true),
	}

	m[ParamMasterUsername] = &MetaData{
		Required: options.Bool(true),
	}

	m[ParamMasterUserPassword] = &MetaData{
		Required: options.Bool(true),
	}

	delete(m, ParamAllowMajorVersionUpgrade)
	return m
}

func GetParametersMetaDataForReadReplicaInstall() map[string]*MetaData {
	return map[string]*MetaData{
		ParamDBInstanceIdentifier: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamSourceDBInstanceIdentifier: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamEngine: {
			Immutable: options.Bool(true),
		},
		ParamDBParameterGroupName: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBInstanceClass: {
			Required:                    options.Bool(true),
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamStorageType: {
			Required: options.Bool(true),
			Default:  options.String("gp2"),
		},
		ParamAllocatedStorage: {
			Required: options.Bool(true),
			Default:  options.String("20"),
		},
		ParamAutoMinorVersionUpgrade: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBSubnetGroupName: {
			Immutable: options.Bool(true),
		},
		ParamMasterUserPassword: {},
		ParamDeletionProtection: {},
		ParamIops:               {},
		ParamMultiAZ: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamPort: {},
		ParamPubliclyAccessible: {
			Default: options.String("false"),
		},
		ParamSubnetIds:         {},
		ParamVPCSecurityGroups: {},
		ParamVPC:               {},
	}
}

func GetParametersMetaDataForRestoreFromSnapshotInstall() map[string]*MetaData {
	return map[string]*MetaData{
		ParamDBInstanceIdentifier: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamDBSnapshotIdentifier: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamEngine: {
			Immutable: options.Bool(true),
		},
		ParamDBParameterGroupName: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBInstanceClass: {
			Required:                    options.Bool(true),
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamStorageType: {
			Required: options.Bool(true),
			Default:  options.String("gp2"),
		},
		ParamAllocatedStorage: {
			Required: options.Bool(true),
			Default:  options.String("20"),
		},
		ParamAutoMinorVersionUpgrade: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamDBSubnetGroupName: {
			Immutable: options.Bool(true),
		},
		ParamDeletionProtection: {},
		ParamIops:               {},
		ParamMultiAZ: {
			UpdatesWithSomeInterruption: options.Bool(true),
		},
		ParamPort: {},
		ParamPubliclyAccessible: {
			Default: options.String("false"),
		},
		ParamSubnetIds:         {},
		ParamVPCSecurityGroups: {},
		ParamVPC:               {},
	}
}
