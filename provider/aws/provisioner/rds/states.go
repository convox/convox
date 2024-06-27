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

	Paramters map[string]Parameter `json:"parameters"`
}

func NewState(id string, state StatusType, params []Parameter) *StateData {
	s := &StateData{
		Id:        id,
		Status:    state,
		Imported:  false,
		Locked:    options.Bool(false),
		Paramters: map[string]Parameter{},
	}

	for _, p := range params {
		s.AddOrUpdateParameter(p)
	}

	return s
}

func NewEmptyState() *StateData {
	return &StateData{
		Paramters: map[string]Parameter{},
	}
}

func NewStateForImport(id string) *StateData {
	return &StateData{
		Id:        id,
		Imported:  true,
		Paramters: map[string]Parameter{},
	}
}

func (s *StateData) GetStatus() string {
	return string(s.Status)
}

func (s *StateData) SetStatus(state StatusType) {
	s.Status = state
}

func (s *StateData) AddParameter(p Parameter) error {
	if _, has := s.Paramters[*p.Key]; has {
		return fmt.Errorf("parameter with same already exists")
	}

	s.Paramters[*p.Key] = p
	return nil
}

func (s *StateData) AddOrUpdateParameter(p Parameter) {
	s.Paramters[*p.Key] = p
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
	ps := []Parameter{}
	for _, p := range s.Paramters {
		ps = append(ps, p)
	}
	return ps
}

func (s *StateData) GetStateInBytes() ([]byte, error) {
	return json.Marshal(s)
}

// it will update the value if it's not immutable,
// also it will return true or false if the given value differs from existing value
func (s *StateData) UpdateParameterValueForDbUpdate(key, value string) (bool, error) {
	updateMetadataMap := GetParametersMetaDataForUpdate()
	param, has := s.Paramters[key]
	if !has {
		// adding new parameter, since this is new parameter key for this state
		m, has := updateMetadataMap[key]
		if !has {
			return false, fmt.Errorf("parameter metadata not found: %s", key)
		}
		s.AddOrUpdateParameter(*NewParameter(key, value, m))
		return true, nil
	}

	isUpdated, err := param.Update(value)
	if err != nil {
		return false, err
	}

	if isUpdated {
		m, has := updateMetadataMap[key]
		if !has {
			return false, fmt.Errorf("parameter metadata not found: %s", key)
		}

		if err := param.UpdateMetaData(m); err != nil {
			return false, err
		}

		s.AddOrUpdateParameter(param)

		return isUpdated, nil
	}
	return isUpdated, nil
}

func (s *StateData) InitializeParameterValue(key, value string) error {
	param, has := s.Paramters[key]
	if !has {
		return fmt.Errorf("parameter not found to initialize: %s", key)
	}

	if err := param.Initialize(value); err != nil {
		return err
	}

	s.AddOrUpdateParameter(param)

	return nil
}

func (s *StateData) LoadState(data []byte) error {
	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to load state: %s", err)
	}
	return nil
}

func (s *StateData) AddParameterForImportByValuePtr(key string, value interface{}) error {
	v, err := convertToStringPtr(value)
	if err != nil {
		return err
	}

	if _, err := s.GetParameterValuePtr(key); err == nil {
		return fmt.Errorf("parameter already exists: %s", key)
	}

	paramsMeta := GetParametersMetaDataForImport()
	m, has := paramsMeta[key]
	if !has {
		return fmt.Errorf("metadata not found for param: %s", key)
	}

	if m.Required != nil && *m.Required && (v == nil || *v == "") {
		return fmt.Errorf("value is required for parameter: %s", key)
	}

	s.AddOrUpdateParameter(*NewParameterForValuePtr(key, v, m))

	return nil
}

func (s *StateData) ImportState(db *rdstypes.DBInstance, newMasterPassword *string) error {
	if err := s.AddParameterForImportByValuePtr(ParamDBInstanceIdentifier, db.DBInstanceIdentifier); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamAllocatedStorage, db.AllocatedStorage); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamAutoMinorVersionUpgrade, db.AutoMinorVersionUpgrade); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamBackupRetentionPeriod, db.BackupRetentionPeriod); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamDBInstanceClass, db.DBInstanceClass); err != nil {
		return err
	}

	dbName := db.DBName
	if db.DBName == nil || *db.DBName == "" {
		dbName = db.Engine
	}

	if err := s.AddParameterForImportByValuePtr(ParamDBName, dbName); err != nil {
		return err
	}

	if len(db.DBParameterGroups) > 0 {
		if err := s.AddParameterForImportByValuePtr(ParamDBParameterGroupName, db.DBParameterGroups[0].DBParameterGroupName); err != nil {
			return err
		}
	}

	if db.DBSubnetGroup != nil {
		if err := s.AddParameterForImportByValuePtr(ParamDBSubnetGroupName, db.DBSubnetGroup.DBSubnetGroupName); err != nil {
			return err
		}
	}

	if err := s.AddParameterForImportByValuePtr(ParamDeletionProtection, db.DeletionProtection); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamEngine, db.Engine); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamEngineVersion, db.EngineVersion); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamIops, db.Iops); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamMasterUsername, db.MasterUsername); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamMasterUserPassword, newMasterPassword); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamMultiAZ, db.MultiAZ); err != nil {
		return err
	}

	if db.Endpoint != nil {
		if err := s.AddParameterForImportByValuePtr(ParamPort, db.Endpoint.Port); err != nil {
			return err
		}
	}

	if err := s.AddParameterForImportByValuePtr(ParamPreferredBackupWindow, db.PreferredBackupWindow); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamPreferredMaintenanceWindow, db.PreferredMaintenanceWindow); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamPubliclyAccessible, db.PubliclyAccessible); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamStorageEncrypted, db.StorageEncrypted); err != nil {
		return err
	}

	if err := s.AddParameterForImportByValuePtr(ParamStorageType, db.StorageType); err != nil {
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
			if err := s.AddParameterForImportByValuePtr(ParamVPCSecurityGroups, sgList); err != nil {
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

	paramsObj.SourceDBInstanceIdentifier, _ = s.GetParameterValuePtr(ParamSourceDBInstanceIdentifier)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.VpcSecurityGroupIds, err = s.GetParameterValueStringArray(ParamVPCSecurityGroups)
	if err != nil {
		return nil, err
	}

	return paramsObj, nil
}

func (s *StateData) ToAllCommonParamsForReplicaInstall() (*AllCommonParams, error) {
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

	paramsObj.DBInstanceClass, err = s.GetParameterValuePtr(ParamDBInstanceClass)
	if err != nil {
		return nil, err
	}

	paramsObj.DBInstanceIdentifier, err = s.GetParameterValuePtr(ParamDBInstanceIdentifier)
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

	paramsObj.Iops, err = s.GetParameterValueInt32Ptr(ParamIops)
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

	paramsObj.PubliclyAccessible, err = s.GetParameterValueBoolPtr(ParamPubliclyAccessible)
	if err != nil {
		return nil, err
	}

	paramsObj.SourceDBInstanceIdentifier, _ = s.GetParameterValuePtr(ParamSourceDBInstanceIdentifier)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}

	paramsObj.VpcSecurityGroupIds, err = s.GetParameterValueStringArray(ParamVPCSecurityGroups)
	if err != nil {
		return nil, err
	}

	return paramsObj, nil
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
	paramsMeta := GetParametersMetaDataForUpdate()
	changedAndRequiredParams := []Parameter{}
	paramMap := map[string]struct{}{}
	for _, paramKey := range changedParams {
		value, err := s.GetParameterValue(paramKey)
		if err != nil && !IsNotFoundError(err) {
			return nil, err
		}

		// probably value is set to empty string
		if IsNotFoundError(err) {
			valuePtr, err2 := s.GetParameterValuePtr(paramKey)
			if err2 != nil {
				return nil, err2
			}

			if valuePtr != nil {
				value = *valuePtr
			} else {
				return nil, err
			}
		}

		m, has := paramsMeta[paramKey]
		if !has {
			return nil, fmt.Errorf("metadata not found for param: %s", paramKey)
		}

		changedAndRequiredParams = append(changedAndRequiredParams, *NewParameter(paramKey, value, m))
		paramMap[paramKey] = struct{}{}
	}

	for pKey, m := range paramsMeta {
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
	allParamsObj, err := s.ToAllCommonParamsForReplicaInstall()
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
