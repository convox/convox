package rds

import (
	"fmt"

	"github.com/convox/convox/pkg/options"
)

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
	ParamImport           = "Import"
)

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

func (p *Parameter) UpdateMetaData(m *MetaData) error {
	if m == nil {
		return fmt.Errorf("meta data is nil")
	}
	p.Meta = m
	return nil
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
		ParamImport,
	}
}
