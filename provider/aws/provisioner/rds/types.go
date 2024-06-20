package rds

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/convox/convox/pkg/options"
)

type MetaData struct {
	Required                    *bool   `json:"required"`
	Ignore                      *bool   `json:"ignore"`
	Immutable                   *bool   `json:"immutable"`
	UpdatesWithSomeInterruption *bool   `json:"updatesWithSomeInterruption"`
	Default                     *string `json:"default"`
}

type Parameter struct {
	Meta  *MetaData `json:"meta"`
	Key   *string   `json:"key"`
	Value *string   `json:"value"`
}

func NewParameter(key, value string, m *MetaData) *Parameter {
	var vPtr *string
	if value == "" && m != nil && m.Default != nil {
		value = *m.Default
	}
	if value != "" {
		vPtr = options.String(value)
	}
	return &Parameter{
		Key:   &key,
		Value: vPtr,
		Meta:  m,
	}
}

func NewParameterForValuePtr(key string, value *string, m *MetaData) *Parameter {
	return &Parameter{
		Key:   &key,
		Value: value,
		Meta:  m,
	}
}

func (p *Parameter) Validate() error {
	if p.Key == nil {
		return fmt.Errorf("param key is not defined")
	}
	if p.Meta == nil {
		return fmt.Errorf("param metadata is not defined for %s", *p.Key)
	}

	if p.Meta.Required != nil && *p.Meta.Required && p.IsValueEmpty() {
		return fmt.Errorf("%s parameter value is required", *p.Key)
	}
	return nil
}

func (p *Parameter) IsValueEmpty() bool {
	return p.Value == nil || *p.Value == ""
}

func (p *Parameter) Initialize(v string) error {
	p.Value = options.String(v)
	return nil
}

func (p *Parameter) Update(v string) (bool, error) {
	if p.Value != nil && *p.Value == v {
		return false, nil
	}

	if p.Meta.Immutable != nil && *p.Meta.Immutable {
		return true, fmt.Errorf("immutable parameter value modification not allowed")
	}

	if v == "" {
		p.Value = nil
	} else {
		p.Value = options.String(v)
	}
	return true, nil
}

func (p *Parameter) GetKey() (string, error) {
	if p.Key == nil {
		return "", fmt.Errorf("key not found")
	}
	return *p.Key, nil
}

func (p *Parameter) GetValue() (string, error) {
	key, err := p.GetKey()
	if err != nil {
		return "", err
	}
	if p.IsValueEmpty() {
		return "", fmt.Errorf("value not found for param: %s", key)
	}
	return *p.Value, nil
}

func (p *Parameter) GetValuePtr() (*string, error) {
	return p.Value, nil
}

func (p *Parameter) IsRequired() (bool, error) {
	if p.Meta == nil {
		return false, fmt.Errorf("param metadata is not found")
	}
	if p.Meta.Required != nil {
		return *p.Meta.Required, nil
	}

	return false, nil
}

type AllCommonParams struct {
	DBInstanceClass                    *string
	DBInstanceIdentifier               *string
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
	DBClusterIdentifier                *string
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

type ConnectionInfo struct {
	Host     string
	Port     string
	UserName string
	Password string
	Database string
}
