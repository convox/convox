package rds

import (
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

type AllCommonParams struct {
	DBInstanceClass                    *string
	DBInstanceIdentifier               *string
	DBSnapshotIdentifier               *string
	Engine                             *string
	AllocatedStorage                   *int32
	AutoMinorVersionUpgrade            *bool
	AvailabilityZone                   *string
	BackupRetentionPeriod              *int32
	BackupTarget                       *string
	CACertificateIdentifier            *string
	CharacterSetName                   *string
	CopyTagsToSnapshot                 *bool
	CustomIamInstanceProfile           *string
	DBName                             *string
	DBParameterGroupName               *string
	DBSecurityGroups                   []string
	DBSubnetGroupName                  *string
	DBSystemId                         *string
	DedicatedLogVolume                 *bool
	DeletionProtection                 *bool
	Domain                             *string
	DomainAuthSecretArn                *string
	DomainDnsIps                       []string
	DomainFqdn                         *string
	DomainIAMRoleName                  *string
	DomainOu                           *string
	EnableCloudwatchLogsExports        []string
	EnableCustomerOwnedIp              *bool
	EnableIAMDatabaseAuthentication    *bool
	EnablePerformanceInsights          *bool
	EngineLifecycleSupport             *string
	EngineVersion                      *string
	Iops                               *int32
	KmsKeyId                           *string
	LicenseModel                       *string
	MasterUserPassword                 *string
	MasterUserSecretKmsKeyId           *string
	MasterUsername                     *string
	MaxAllocatedStorage                *int32
	MonitoringInterval                 *int32
	MonitoringRoleArn                  *string
	MultiAZ                            *bool
	MultiTenant                        *bool
	NcharCharacterSetName              *string
	NetworkType                        *string
	OptionGroupName                    *string
	PerformanceInsightsKMSKeyId        *string
	PerformanceInsightsRetentionPeriod *int32
	Port                               *int32
	PreferredBackupWindow              *string
	PreferredMaintenanceWindow         *string
	ProcessorFeatures                  []types.ProcessorFeature
	PromotionTier                      *int32
	PubliclyAccessible                 *bool
	StorageEncrypted                   *bool
	StorageThroughput                  *int32
	StorageType                        *string
	Tags                               []types.Tag
	SourceDBInstanceIdentifier         *string
	TdeCredentialArn                   *string
	TdeCredentialPassword              *string
	Timezone                           *string
	VpcSecurityGroupIds                []string
}