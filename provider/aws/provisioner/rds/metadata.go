package rds

import (
	"fmt"

	"github.com/convox/convox/pkg/options"
)

func ParameterMetaDataForInstall(name string) (*MetaData, error) {
	switch name {
	case ParamDBInstanceIdentifier:
		return &MetaData{
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		}, nil
	case ParamDBName:
		return &MetaData{
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
			Default:   options.String("app"),
		}, nil
	case ParamDBParameterGroupName:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamEngine:
		return &MetaData{
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		}, nil
	case ParamEngineVersion:
		return &MetaData{
			Required:                    options.Bool(true),
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamDBInstanceClass:
		return &MetaData{
			Required:                    options.Bool(true),
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamStorageType:
		return &MetaData{
			Required: options.Bool(true),
			Default:  options.String("gp2"),
		}, nil
	case ParamAllocatedStorage:
		return &MetaData{
			Required: options.Bool(true),
			Default:  options.String("20"),
		}, nil
	case ParamMasterUsername:
		return &MetaData{
			Required: options.Bool(true),
			Default:  options.String("app"),
		}, nil
	case ParamMasterUserPassword:
		return &MetaData{
			Required: options.Bool(true),
		}, nil
	case ParamAllowMajorVersionUpgrade:
		return &MetaData{
			Default: options.String("true"),
		}, nil
	case ParamAutoMinorVersionUpgrade:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamBackupRetentionPeriod:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamDBSubnetGroupName:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamDBSnapshotIdentifier:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamDeletionProtection:
		return &MetaData{}, nil
	case ParamIops:
		return &MetaData{}, nil
	case ParamMultiAZ:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamPort:
		return &MetaData{
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		}, nil
	case ParamPreferredBackupWindow:
		return &MetaData{}, nil
	case ParamPreferredMaintenanceWindow:
		return &MetaData{}, nil
	case ParamPubliclyAccessible:
		return &MetaData{
			Default: options.String("false"),
		}, nil
	case ParamStorageEncrypted:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamSourceDBInstanceIdentifier:
		return &MetaData{}, nil
	case ParamSubnetIds:
		return &MetaData{}, nil
	case ParamVPCSecurityGroups:
		return &MetaData{}, nil
	case ParamVPC:
		return &MetaData{}, nil
	default:
		return &MetaData{}, nil
	}
}

func ParameterMetaDataForUpdate(name string) (*MetaData, error) {
	switch name {
	case ParamDBInstanceIdentifier:
		return &MetaData{
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		}, nil
	case ParamDBName:
		return &MetaData{
			Immutable: options.Bool(true),
			Default:   options.String("app"),
		}, nil
	case ParamDBParameterGroupName:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamEngine:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamEngineVersion:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamDBInstanceClass:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamStorageType:
		return &MetaData{
			Default: options.String("gp2"),
		}, nil
	case ParamAllocatedStorage:
		return &MetaData{
			Default: options.String("20"),
		}, nil
	case ParamMasterUsername:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamMasterUserPassword:
		return &MetaData{}, nil
	case ParamAllowMajorVersionUpgrade:
		return &MetaData{
			Default: options.String("true"),
		}, nil
	case ParamAutoMinorVersionUpgrade:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamBackupRetentionPeriod:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamDBSubnetGroupName:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamDBSnapshotIdentifier:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamDeletionProtection:
		return &MetaData{}, nil
	case ParamIops:
		return &MetaData{}, nil
	case ParamMultiAZ:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamPort:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamPreferredBackupWindow:
		return &MetaData{}, nil
	case ParamPreferredMaintenanceWindow:
		return &MetaData{}, nil
	case ParamPubliclyAccessible:
		return &MetaData{
			Default: options.String("false"),
		}, nil
	case ParamStorageEncrypted:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamSourceDBInstanceIdentifier:
		return &MetaData{
			UpdatesWithSomeInterruption: options.Bool(true),
		}, nil
	case ParamSubnetIds:
		return &MetaData{}, nil
	case ParamVPCSecurityGroups:
		return &MetaData{
			Immutable: options.Bool(true),
		}, nil
	case ParamVPC:
		return &MetaData{}, nil
	case ParamApplyImmediately:
		return &MetaData{
			Default: options.String("true"),
		}, nil
	default:
		return nil, fmt.Errorf("metadata not found for param: %s", name)
	}
}

func ParameterMetaDataForImport(name string) (*MetaData, error) {
	switch name {
	case ParamDBInstanceIdentifier:
		return &MetaData{
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		}, nil
	case ParamAllowMajorVersionUpgrade:
		return nil, fmt.Errorf("not supported")
	default:
		return &MetaData{}, nil
	}
}
