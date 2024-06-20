package rds

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/convox/convox/pkg/options"
)

type StatusType string

const (
	StateProvisioning       StatusType = "provisioning"
	StateProvisionSucceeded StatusType = "provision_succeeded"
	StateProvisionFailed    StatusType = "provision_failed"
	StateUpdating           StatusType = "updating"
	StateUpdateSucceeded    StatusType = "update_succeeded"
)

type StateData struct {
	Id     string     `json:"id"`
	Status StatusType `json:"state"`

	Imported bool `json:"imported"`

	Locked       *bool  `json:"locked"`
	LockedReason string `json:"lockedReason"`

	Host string `json:"host"`

	Paramters []Parameter `json:"parameters"`
}

func NewState(id string, state StatusType, params []Parameter) *StateData {
	return &StateData{
		Id:        id,
		Status:    state,
		Imported:  false,
		Locked:    options.Bool(false),
		Paramters: params,
	}
}

func NewEmptyState() *StateData {
	return &StateData{}
}

func NewStateForImport(id string) *StateData {
	return &StateData{
		Id:        id,
		Imported:  true,
		Paramters: []Parameter{},
	}
}

func (s *StateData) GetStatus() string {
	return string(s.Status)
}

func (s *StateData) SetStatus(state StatusType) {
	s.Status = state
}

func (s *StateData) GetParameter(key string) (*Parameter, error) {
	for _, p := range s.Paramters {
		k, err := p.GetKey()
		if err != nil {
			return nil, err
		}
		if k == key {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("parameter not found: %s", key)
}

func (s *StateData) GetParameterValue(key string) (string, error) {
	for _, p := range s.Paramters {
		k, err := p.GetKey()
		if err != nil {
			return "", err
		}
		if k == key {
			return p.GetValue()
		}
	}
	return "", fmt.Errorf("parameter not found: %s", key)
}

func (s *StateData) GetParameterValuePtr(key string) (*string, error) {
	for _, p := range s.Paramters {
		k, err := p.GetKey()
		if err != nil {
			return nil, err
		}
		if k == key {
			return p.GetValuePtr()
		}
	}
	return nil, fmt.Errorf("parameter not found: %s", key)
}

func (s *StateData) GetParameterValueStringArray(key string) ([]string, error) {
	v, err := s.GetParameterValuePtr(key)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, nil
	}

	return convertToStringArray(*v), nil
}

func (s *StateData) GetParameterValueInt32Ptr(key string) (*int32, error) {
	v, err := s.GetParameterValuePtr(key)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, nil
	}

	vint64, err := strconv.ParseInt(*v, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse value for param %s: %s", key, err)
	}
	vint32 := int32(vint64)
	return &vint32, nil
}

func (s *StateData) GetParameterValueBoolPtr(key string) (*bool, error) {
	v, err := s.GetParameterValuePtr(key)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, nil
	}

	vBool, err := strconv.ParseBool(*v)
	if err != nil {
		return nil, fmt.Errorf("failed to parse value for param %s: %s", key, err)
	}
	return &vBool, nil
}

func (s *StateData) IsInProgress() bool {
	switch s.Status {
	case StateProvisioning, StateUpdating:
		return true
	default:
		return false
	}
}

func (s *StateData) IsLocked() (bool, string) {
	if s.Locked != nil {
		return *s.Locked, s.LockedReason
	}
	return false, s.LockedReason
}

func (s *StateData) Lock(reason string) error {
	if locked, lockReason := s.IsLocked(); locked {
		return fmt.Errorf("state is already locked for reason: %s", lockReason)
	}
	s.Locked = options.Bool(true)
	s.LockedReason = reason
	return nil
}

func (s *StateData) Unlock() error {
	s.Locked = options.Bool(false)
	return nil
}

func (s *StateData) GetParameters() []Parameter {
	return s.Paramters
}

func (s *StateData) GetStateInBytes() ([]byte, error) {
	return json.Marshal(s)
}

// it will update the value if it's not immutable,
// also it will return true or false if the given value differs from existing value
func (s *StateData) UpdateParameterValue(key, value string) (bool, error) {
	for i := range s.Paramters {
		if k, err := s.Paramters[i].GetKey(); err == nil && k == key {
			return s.Paramters[i].Update(value)
		}
	}
	return false, fmt.Errorf("parameter not found: %s", key)
}

func (s *StateData) InitializeParameterValue(key, value string) error {
	for i := range s.Paramters {
		if k, err := s.Paramters[i].GetKey(); err == nil && k == key {
			return s.Paramters[i].Initialize(value)
		}
	}
	return fmt.Errorf("parameter not found: %s", key)
}

func (s *StateData) LoadState(data []byte) error {
	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to load state: %s", err)
	}
	return nil
}

func (s *StateData) AddParameterByValuePtr(key string, value interface{}) error {
	v, err := convertToStringPtr(value)
	if err != nil {
		return err
	}

	if _, err := s.GetParameterValuePtr(key); err == nil {
		return fmt.Errorf("parameter already exists: %s", key)
	}

	m, err := ParameterMetaDataForImport(key)
	if err != nil {
		return err
	}
	if m.Required != nil && *m.Required && value == nil {
		return fmt.Errorf("value is required for parameter: %s", key)
	}
	s.Paramters = append(s.Paramters, *NewParameterForValuePtr(key, v, m))
	return nil
}

func (s *StateData) ImportState(db *rdstypes.DBInstance, newMasterPassword *string) error {
	if err := s.AddParameterByValuePtr(ParamDBInstanceIdentifier, db.DBInstanceIdentifier); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamAllocatedStorage, db.AllocatedStorage); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamAutoMinorVersionUpgrade, db.AutoMinorVersionUpgrade); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamBackupRetentionPeriod, db.BackupRetentionPeriod); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamDBInstanceClass, db.DBInstanceClass); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamDBName, db.DBName); err != nil {
		return err
	}

	if len(db.DBParameterGroups) > 0 {
		if err := s.AddParameterByValuePtr(ParamDBParameterGroupName, db.DBParameterGroups[0].DBParameterGroupName); err != nil {
			return err
		}
	}

	if db.DBSubnetGroup != nil {
		if err := s.AddParameterByValuePtr(ParamDBSubnetGroupName, db.DBSubnetGroup.DBSubnetGroupName); err != nil {
			return err
		}
	}

	if err := s.AddParameterByValuePtr(ParamDeletionProtection, db.DeletionProtection); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamEngine, db.Engine); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamEngineVersion, db.EngineVersion); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamIops, db.Iops); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamMasterUsername, db.MasterUsername); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamMasterUserPassword, newMasterPassword); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamMultiAZ, db.MultiAZ); err != nil {
		return err
	}

	if db.Endpoint != nil {
		if err := s.AddParameterByValuePtr(ParamPort, db.Endpoint.Port); err != nil {
			return err
		}
	}

	if err := s.AddParameterByValuePtr(ParamPreferredBackupWindow, db.PreferredBackupWindow); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamPreferredMaintenanceWindow, db.PreferredMaintenanceWindow); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamPubliclyAccessible, db.PubliclyAccessible); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamStorageEncrypted, db.StorageEncrypted); err != nil {
		return err
	}

	if err := s.AddParameterByValuePtr(ParamStorageType, db.StorageType); err != nil {
		return err
	}

	if len(db.VpcSecurityGroups) > 0 {
		sgList := []string{}
		for _, sg := range db.VpcSecurityGroups {
			if sg.VpcSecurityGroupId != nil {
				sgList = append(sgList, *sg.VpcSecurityGroupId)
			}
		}
		if len(sgList) > 0 {
			if err := s.AddParameterByValuePtr(ParamVPCSecurityGroups, sgList); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *StateData) ToAllCommonParamsForInstall() (*AllCommonParams, error) {
	var err error
	paramsObj := &AllCommonParams{}

	paramsObj.AllocatedStorage, err = s.GetParameterValueInt32Ptr(ParamAllocatedStorage)
	if err != nil {
		return nil, err
	}

	paramsObj.AutoMinorVersionUpgrade, err = s.GetParameterValueBoolPtr(ParamAutoMinorVersionUpgrade)
	if err != nil {
		return nil, err
	}

	paramsObj.BackupRetentionPeriod, err = s.GetParameterValueInt32Ptr(ParamBackupRetentionPeriod)
	if err != nil {
		return nil, err
	}

	paramsObj.DBInstanceClass, err = s.GetParameterValuePtr(ParamDBInstanceClass)
	if err != nil {
		return nil, err
	}

	paramsObj.DBInstanceIdentifier, err = s.GetParameterValuePtr(ParamDBInstanceIdentifier)
	if err != nil {
		return nil, err
	}

	paramsObj.DBName, err = s.GetParameterValuePtr(ParamDBName)
	if err != nil {
		return nil, err
	}

	paramsObj.DBParameterGroupName, err = s.GetParameterValuePtr(ParamDBParameterGroupName)
	if err != nil {
		return nil, err
	}

	paramsObj.DBSubnetGroupName, err = s.GetParameterValuePtr(ParamDBSubnetGroupName)
	if err != nil {
		return nil, err
	}

	paramsObj.DeletionProtection, err = s.GetParameterValueBoolPtr(ParamDeletionProtection)
	if err != nil {
		return nil, err
	}

	paramsObj.Engine, err = s.GetParameterValuePtr(ParamEngine)
	if err != nil {
		return nil, err
	}

	paramsObj.EngineVersion, err = s.GetParameterValuePtr(ParamEngineVersion)
	if err != nil {
		return nil, err
	}

	paramsObj.Iops, err = s.GetParameterValueInt32Ptr(ParamIops)
	if err != nil {
		return nil, err
	}

	paramsObj.MasterUserPassword, err = s.GetParameterValuePtr(ParamMasterUserPassword)
	if err != nil {
		return nil, err
	}

	paramsObj.MasterUsername, err = s.GetParameterValuePtr(ParamMasterUsername)
	if err != nil {
		return nil, err
	}

	paramsObj.MultiAZ, err = s.GetParameterValueBoolPtr(ParamMultiAZ)
	if err != nil {
		return nil, err
	}

	paramsObj.Port, err = s.GetParameterValueInt32Ptr(ParamPort)
	if err != nil {
		return nil, err
	}

	paramsObj.PreferredBackupWindow, err = s.GetParameterValuePtr(ParamPreferredBackupWindow)
	if err != nil {
		return nil, err
	}

	paramsObj.PreferredMaintenanceWindow, err = s.GetParameterValuePtr(ParamPreferredMaintenanceWindow)
	if err != nil {
		return nil, err
	}

	paramsObj.PubliclyAccessible, err = s.GetParameterValueBoolPtr(ParamPubliclyAccessible)
	if err != nil {
		return nil, err
	}

	paramsObj.StorageEncrypted, err = s.GetParameterValueBoolPtr(ParamStorageEncrypted)
	if err != nil {
		return nil, err
	}

	paramsObj.StorageType, err = s.GetParameterValuePtr(ParamStorageType)
	if err != nil {
		return nil, err
	}

	paramsObj.SourceDBInstanceIdentifier, err = s.GetParameterValuePtr(ParamSourceDBInstanceIdentifier)
	if err != nil {
		return nil, err
	}

	paramsObj.VpcSecurityGroupIds, err = s.GetParameterValueStringArray(ParamVPCSecurityGroups)
	if err != nil {
		return nil, err
	}

	return paramsObj, nil
}

func (s *StateData) GenerateCreateDBInstanceInput() (*rds.CreateDBInstanceInput, error) {
	allParamsObj, err := s.ToAllCommonParamsForInstall()
	if err != nil {
		return nil, err
	}
	return &rds.CreateDBInstanceInput{
		AllocatedStorage:           allParamsObj.AllocatedStorage,
		AutoMinorVersionUpgrade:    allParamsObj.AutoMinorVersionUpgrade,
		BackupRetentionPeriod:      allParamsObj.BackupRetentionPeriod,
		DBInstanceClass:            allParamsObj.DBInstanceClass,
		DBInstanceIdentifier:       allParamsObj.DBInstanceIdentifier,
		DBName:                     allParamsObj.DBName,
		DBParameterGroupName:       allParamsObj.DBParameterGroupName,
		DBSubnetGroupName:          allParamsObj.DBSubnetGroupName,
		DeletionProtection:         allParamsObj.DeletionProtection,
		Engine:                     allParamsObj.Engine,
		EngineVersion:              allParamsObj.EngineVersion,
		Iops:                       allParamsObj.Iops,
		MasterUserPassword:         allParamsObj.MasterUserPassword,
		MasterUsername:             allParamsObj.MasterUsername,
		MultiAZ:                    allParamsObj.MultiAZ,
		Port:                       allParamsObj.Port,
		PreferredBackupWindow:      allParamsObj.PreferredBackupWindow,
		PreferredMaintenanceWindow: allParamsObj.PreferredMaintenanceWindow,
		PubliclyAccessible:         allParamsObj.PubliclyAccessible,
		StorageEncrypted:           allParamsObj.StorageEncrypted,
		StorageType:                allParamsObj.StorageType,
		VpcSecurityGroupIds:        allParamsObj.VpcSecurityGroupIds,
	}, nil
}

func (s *StateData) GenerateCreateDBInstanceReadReplicaInput() (*rds.CreateDBInstanceReadReplicaInput, error) {
	allParamsObj, err := s.ToAllCommonParamsForInstall()
	if err != nil {
		return nil, err
	}

	return &rds.CreateDBInstanceReadReplicaInput{
		AllocatedStorage:           allParamsObj.AllocatedStorage,
		AutoMinorVersionUpgrade:    allParamsObj.AutoMinorVersionUpgrade,
		DBInstanceClass:            allParamsObj.DBInstanceClass,
		DBInstanceIdentifier:       allParamsObj.DBInstanceIdentifier,
		DBParameterGroupName:       allParamsObj.DBParameterGroupName,
		DBSubnetGroupName:          allParamsObj.DBSubnetGroupName,
		DeletionProtection:         allParamsObj.DeletionProtection,
		Iops:                       allParamsObj.Iops,
		MultiAZ:                    allParamsObj.MultiAZ,
		Port:                       allParamsObj.Port,
		PubliclyAccessible:         allParamsObj.PubliclyAccessible,
		StorageType:                allParamsObj.StorageType,
		SourceDBInstanceIdentifier: allParamsObj.SourceDBInstanceIdentifier,
		VpcSecurityGroupIds:        allParamsObj.VpcSecurityGroupIds,
	}, nil
}

func (s *StateData) ToAllCommonParamsForUpdate() (*AllCommonParams, error) {
	var err error
	paramsObj := &AllCommonParams{}

	paramsObj.AllocatedStorage, err = s.GetParameterValueInt32Ptr(ParamAllocatedStorage)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.AutoMinorVersionUpgrade, err = s.GetParameterValueBoolPtr(ParamAutoMinorVersionUpgrade)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.BackupRetentionPeriod, err = s.GetParameterValueInt32Ptr(ParamBackupRetentionPeriod)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.DBInstanceClass, err = s.GetParameterValuePtr(ParamDBInstanceClass)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.DBInstanceIdentifier, err = s.GetParameterValuePtr(ParamDBInstanceIdentifier)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.DBName, err = s.GetParameterValuePtr(ParamDBName)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.DBParameterGroupName, err = s.GetParameterValuePtr(ParamDBParameterGroupName)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.DBSubnetGroupName, err = s.GetParameterValuePtr(ParamDBSubnetGroupName)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.DeletionProtection, err = s.GetParameterValueBoolPtr(ParamDeletionProtection)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.Engine, err = s.GetParameterValuePtr(ParamEngine)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.EngineVersion, err = s.GetParameterValuePtr(ParamEngineVersion)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.Iops, err = s.GetParameterValueInt32Ptr(ParamIops)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.MasterUserPassword, err = s.GetParameterValuePtr(ParamMasterUserPassword)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.MasterUsername, err = s.GetParameterValuePtr(ParamMasterUsername)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.MultiAZ, err = s.GetParameterValueBoolPtr(ParamMultiAZ)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.Port, err = s.GetParameterValueInt32Ptr(ParamPort)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.PreferredBackupWindow, err = s.GetParameterValuePtr(ParamPreferredBackupWindow)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.PreferredMaintenanceWindow, err = s.GetParameterValuePtr(ParamPreferredMaintenanceWindow)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.PubliclyAccessible, err = s.GetParameterValueBoolPtr(ParamPubliclyAccessible)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.StorageEncrypted, err = s.GetParameterValueBoolPtr(ParamStorageEncrypted)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.StorageType, err = s.GetParameterValuePtr(ParamStorageType)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.SourceDBInstanceIdentifier, err = s.GetParameterValuePtr(ParamSourceDBInstanceIdentifier)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.VpcSecurityGroupIds, err = s.GetParameterValueStringArray(ParamVPCSecurityGroups)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	return paramsObj, nil
}

func (s *StateData) GenerateModifyDBInstanceInput(changedParams []string) (*rds.ModifyDBInstanceInput, error) {
	changedAndRequiredParams := []Parameter{}
	paramMap := map[string]struct{}{}
	for _, paramKey := range changedParams {
		value, err := s.GetParameterValue(paramKey)
		if err != nil {
			return nil, err
		}

		m, err := ParameterMetaDataForUpdate(paramKey)
		if err != nil {
			return nil, err
		}

		changedAndRequiredParams = append(changedAndRequiredParams, *NewParameter(paramKey, value, m))
		paramMap[paramKey] = struct{}{}
	}

	allParams := ParametersNameList()
	for _, pKey := range allParams {
		m, err := ParameterMetaDataForUpdate(pKey)
		if err != nil {
			if IsNotFoundError(err) {
				continue
			}
			return nil, err
		}

		if m.Required != nil && *m.Required {
			if _, has := paramMap[pKey]; !has {
				value, err := s.GetParameterValue(pKey)
				if err != nil {
					return nil, err
				}

				changedAndRequiredParams = append(changedAndRequiredParams, *NewParameter(pKey, value, m))
				paramMap[pKey] = struct{}{}
			}
		}
	}

	changedStateData := NewState(s.Id, StateUpdating, changedAndRequiredParams)

	allParamsObj, err := changedStateData.ToAllCommonParamsForUpdate()
	if err != nil {
		return nil, err
	}

	return &rds.ModifyDBInstanceInput{
		AllocatedStorage:           allParamsObj.AllocatedStorage,
		AutoMinorVersionUpgrade:    allParamsObj.AutoMinorVersionUpgrade,
		BackupRetentionPeriod:      allParamsObj.BackupRetentionPeriod,
		DBInstanceClass:            allParamsObj.DBInstanceClass,
		DBInstanceIdentifier:       allParamsObj.DBInstanceIdentifier,
		DBParameterGroupName:       allParamsObj.DBParameterGroupName,
		DBSubnetGroupName:          allParamsObj.DBSubnetGroupName,
		DeletionProtection:         allParamsObj.DeletionProtection,
		Engine:                     allParamsObj.Engine,
		EngineVersion:              allParamsObj.EngineVersion,
		Iops:                       allParamsObj.Iops,
		MasterUserPassword:         allParamsObj.MasterUserPassword,
		MultiAZ:                    allParamsObj.MultiAZ,
		PreferredBackupWindow:      allParamsObj.PreferredBackupWindow,
		PreferredMaintenanceWindow: allParamsObj.PreferredMaintenanceWindow,
		PubliclyAccessible:         allParamsObj.PubliclyAccessible,
		StorageType:                allParamsObj.StorageType,
	}, nil
}
