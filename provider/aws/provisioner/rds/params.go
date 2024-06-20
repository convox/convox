package rds

type RDSParameters map[string]*Parameter

const (
	ParamDBInstanceIdentifier       = "DBInstanceIdentifier"
	ParamDBName                     = "DBName"
	ParamDBParameterGroupName       = "DBParameterGroupName"
	ParamEngine                     = "Engine"
	ParamEngineVersion              = "EngineVersion"
	ParamDBInstanceClass            = "DBInstanceClass"
	ParamStorageType                = "StorageType"
	ParamAllocatedStorage           = "AllocatedStorage"
	ParamMasterUsername             = "MasterUsername"
	ParamMasterUserPassword         = "MasterUserPassword"
	ParamAllowMajorVersionUpgrade   = "AllowMajorVersionUpgrade"
	ParamAutoMinorVersionUpgrade    = "AutoMinorVersionUpgrade"
	ParamBackupRetentionPeriod      = "BackupRetentionPeriod"
	ParamDBSubnetGroupName          = "DBSubnetGroupName"
	ParamDBSnapshotIdentifier       = "DBSnapshotIdentifier"
	ParamDeletionProtection         = "DeletionProtection"
	ParamIops                       = "Iops"
	ParamMultiAZ                    = "MultiAZ"
	ParamPort                       = "Port"
	ParamPreferredBackupWindow      = "PreferredBackupWindow"
	ParamPreferredMaintenanceWindow = "PreferredMaintenanceWindow"
	ParamPubliclyAccessible         = "PubliclyAccessible"
	ParamStorageEncrypted           = "StorageEncrypted"
	ParamSourceDBInstanceIdentifier = "SourceDBInstanceIdentifier"
	ParamVPCSecurityGroups          = "VPCSecurityGroups"

	// custom defined params
	ParamSubnetIds        = "SubnetIds"        // used to create db subnet group
	ParamVPC              = "VPC"              // used to create db subnet group and security groups
	ParamApplyImmediately = "ApplyImmediately" // to apply changes immediately or it will apply on the next maintainance window
)

func ParametersNameList() []string {
	return []string{
		ParamDBInstanceIdentifier,
		ParamDBName,
		ParamDBParameterGroupName,
		ParamEngine,
		ParamEngineVersion,
		ParamDBInstanceClass,
		ParamStorageType,
		ParamAllocatedStorage,
		ParamMasterUsername,
		ParamMasterUserPassword,
		ParamAllowMajorVersionUpgrade,
		ParamAutoMinorVersionUpgrade,
		ParamBackupRetentionPeriod,
		ParamDBSubnetGroupName,
		ParamDBSnapshotIdentifier,
		ParamDeletionProtection,
		ParamIops,
		ParamMultiAZ,
		ParamPort,
		ParamPreferredBackupWindow,
		ParamPreferredMaintenanceWindow,
		ParamPubliclyAccessible,
		ParamStorageEncrypted,
		ParamSourceDBInstanceIdentifier,
		ParamSubnetIds,
		ParamVPCSecurityGroups,
		ParamVPC,
		ParamApplyImmediately,
	}
}
