package elasticache

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/provider/aws/provisioner"
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

func (s *StateData) GetParameter(key string) *Parameter {
	p, has := s.Paramters[key]
	if !has {
		return nil
	}
	return &p
}

func (s *StateData) GetParameterValue(key string) (string, error) {
	p := s.GetParameter(key)
	if p == nil {
		return "", fmt.Errorf("parameter not found: %s", key)
	}
	return p.GetValue()
}

func (s *StateData) GetParameterValuePtr(key string) (*string, error) {
	p := s.GetParameter(key)
	if p == nil {
		return nil, fmt.Errorf("parameter not found: %s", key)
	}
	return p.GetValuePtr()
}

func (s *StateData) GetParameterValueStringArray(key string) ([]string, error) {
	v, err := s.GetParameterValuePtr(key)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, nil
	}

	return provisioner.ConvertToStringArray(*v), nil
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
func (s *StateData) UpdateParameterValueForCacheUpdate(key, value string, updateMetadataMap map[string]*MetaData) (bool, error) {
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

func (s *StateData) AddParameterForImportByValuePtr(key string, value interface{}, paramsMeta map[string]*MetaData) error {
	v, err := provisioner.ConvertToStringPtr(value)
	if err != nil {
		return err
	}

	if _, err := s.GetParameterValuePtr(key); err == nil {
		return fmt.Errorf("parameter already exists: %s", key)
	}

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

func (s *StateData) GenerateCreateReplicationGroupInput() (*elasticache.CreateReplicationGroupInput, error) {
	input := &elasticache.CreateReplicationGroupInput{}

	var err error
	input.ReplicationGroupId, err = s.GetParameterValuePtr(ParamReplicationGroupId)
	if err != nil {
		return nil, err
	}

	input.ReplicationGroupDescription, err = s.GetParameterValuePtr(ParamReplicationGroupDescription)
	if err != nil {
		return nil, err
	}

	input.AtRestEncryptionEnabled, err = s.GetParameterValueBoolPtr(ParamAtRestEncryptionEnabled)
	if err != nil {
		return nil, err
	}

	input.AuthToken, err = s.GetParameterValuePtr(ParamAuthToken)
	if err != nil {
		return nil, err
	}
	if input.AuthToken != nil && (len(*input.AuthToken) < 16 || len(*input.AuthToken) > 128) {
		return nil, fmt.Errorf("invalid auth token (password): Must be only printable ASCII characters, Must be at least 16 characters and no more than 128 characters in length, the only permitted printable special characters are !, &, #, $, ^, <, >")
	}

	input.AutoMinorVersionUpgrade, err = s.GetParameterValueBoolPtr(ParamAutoMinorVersionUpgrade)
	if err != nil {
		return nil, err
	}

	input.AutomaticFailoverEnabled, err = s.GetParameterValueBoolPtr(ParamAutomaticFailoverEnabled)
	if err != nil {
		return nil, err
	}

	input.CacheNodeType, err = s.GetParameterValuePtr(ParamCacheNodeType)
	if err != nil {
		return nil, err
	}

	input.CacheParameterGroupName, err = s.GetParameterValuePtr(ParamCacheParameterGroupName)
	if err != nil {
		return nil, err
	}

	input.CacheSubnetGroupName, err = s.GetParameterValuePtr(ParamCacheSubnetGroupName)
	if err != nil {
		return nil, err
	}

	input.Engine, err = s.GetParameterValuePtr(ParamEngine)
	if err != nil {
		return nil, err
	}

	input.EngineVersion, err = s.GetParameterValuePtr(ParamEngineVersion)
	if err != nil {
		return nil, err
	}

	input.NetworkType = elasticachetypes.NetworkTypeIpv4

	input.NumCacheClusters, err = s.GetParameterValueInt32Ptr(ParamNumCacheClusters)
	if err != nil {
		return nil, err
	}

	input.Port, err = s.GetParameterValueInt32Ptr(ParamPort)
	if err != nil {
		return nil, err
	}

	input.SecurityGroupIds, err = s.GetParameterValueStringArray(ParamSecurityGroupIds)
	if err != nil {
		return nil, err
	}

	input.TransitEncryptionEnabled, err = s.GetParameterValueBoolPtr(ParamTransitEncryptionEnabled)
	if err != nil {
		return nil, err
	}

	transitMode, err := s.GetParameterValuePtr(ParamTransitEncryptionMode)
	if err != nil {
		return nil, err
	}

	if transitMode != nil {
		if !provisioner.TargetExistsInStringArray([]string{
			string(elasticachetypes.TransitEncryptionModePreferred),
			string(elasticachetypes.TransitEncryptionModeRequired),
		}, *transitMode) {
			return nil, fmt.Errorf("invalid value in the parameter '%s'", ParamTransitEncryptionMode)
		}

		input.TransitEncryptionMode = elasticachetypes.TransitEncryptionMode(*transitMode)
	}

	return input, nil
}

func (s *StateData) GenerateModifyReplicationGroupInput(changedParams []string) (*elasticache.ModifyReplicationGroupInput, error) {
	modifiedState := NewEmptyState()
	updateMetadata := GetParametersMetaDataForReplicationGroupUpdate()
	for k, m := range updateMetadata {
		if m.Required != nil && *m.Required {
			param := s.GetParameter(k)
			if param == nil {
				return nil, fmt.Errorf("required param '%s' not found in the state", k)
			}
			modifiedState.AddOrUpdateParameter(*param)
		} else if provisioner.TargetExistsInStringArray(changedParams, k) {
			param := s.GetParameter(k)
			if param == nil {
				return nil, fmt.Errorf("changed param '%s' not found in the state", k)
			}
			modifiedState.AddOrUpdateParameter(*param)
		}
	}

	input := &elasticache.ModifyReplicationGroupInput{}

	var err error
	input.ReplicationGroupId, err = modifiedState.GetParameterValuePtr(ParamReplicationGroupId)
	if err != nil {
		return nil, err
	}

	input.ApplyImmediately, err = modifiedState.GetParameterValueBoolPtr(ParamApplyImmediately)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.AuthToken, err = modifiedState.GetParameterValuePtr(ParamAuthToken)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	if input.AuthToken != nil && (len(*input.AuthToken) < 16 || len(*input.AuthToken) > 128) {
		return nil, fmt.Errorf("invalid auth token (password): Must be only printable ASCII characters, Must be at least 16 characters and no more than 128 characters in length, the only permitted printable special characters are !, &, #, $, ^, <, >")
	}

	input.AutoMinorVersionUpgrade, err = modifiedState.GetParameterValueBoolPtr(ParamAutoMinorVersionUpgrade)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.AutomaticFailoverEnabled, err = modifiedState.GetParameterValueBoolPtr(ParamAutomaticFailoverEnabled)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.CacheNodeType, err = modifiedState.GetParameterValuePtr(ParamCacheNodeType)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.CacheParameterGroupName, err = modifiedState.GetParameterValuePtr(ParamCacheParameterGroupName)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.EngineVersion, err = modifiedState.GetParameterValuePtr(ParamEngineVersion)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.SecurityGroupIds, err = modifiedState.GetParameterValueStringArray(ParamSecurityGroupIds)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.TransitEncryptionEnabled, err = modifiedState.GetParameterValueBoolPtr(ParamTransitEncryptionEnabled)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	transitMode, err := s.GetParameterValuePtr(ParamTransitEncryptionMode)
	if err != nil {
		return nil, err
	}

	if transitMode != nil {
		if !provisioner.TargetExistsInStringArray([]string{
			string(elasticachetypes.TransitEncryptionModePreferred),
			string(elasticachetypes.TransitEncryptionModeRequired),
		}, *transitMode) {
			return nil, fmt.Errorf("invalid value in the parameter '%s'", ParamTransitEncryptionMode)
		}

		input.TransitEncryptionMode = elasticachetypes.TransitEncryptionMode(*transitMode)
	}

	return input, nil
}

func (s *StateData) ImportReplicationGroup(r *elasticachetypes.ReplicationGroup, authToken *string) error {
	m := GetParametersMetaDataForReplicationGroupImport()

	err := s.AddParameterForImportByValuePtr(ParamReplicationGroupId, r.ReplicationGroupId, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamAtRestEncryptionEnabled, r.AtRestEncryptionEnabled, m)
	if err != nil {
		return err
	}

	if r.AuthTokenEnabled != nil && *r.AuthTokenEnabled {
		if authToken == nil {
			return fmt.Errorf("auth token is required, since it is enabled in the replication group")
		}
		err = s.AddParameterForImportByValuePtr(ParamAuthToken, authToken, m)
		if err != nil {
			return err
		}
	}

	err = s.AddParameterForImportByValuePtr(ParamAutoMinorVersionUpgrade, r.AutoMinorVersionUpgrade, m)
	if err != nil {
		return err
	}

	automaticFailover := false
	if r.AutomaticFailover == elasticachetypes.AutomaticFailoverStatusEnabled || r.AutomaticFailover == elasticachetypes.AutomaticFailoverStatusEnabling {
		automaticFailover = true
	}

	err = s.AddParameterForImportByValuePtr(ParamAutomaticFailoverEnabled, automaticFailover, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamCacheNodeType, r.CacheNodeType, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamEngine, "redis", m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamReplicationGroupDescription, r.Description, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamTransitEncryptionEnabled, r.TransitEncryptionEnabled, m)
	if err != nil {
		return err
	}

	return nil
}

func (s *StateData) GenerateCreateCacheClusterInput() (*elasticache.CreateCacheClusterInput, error) {
	input := &elasticache.CreateCacheClusterInput{}

	var err error
	input.CacheClusterId, err = s.GetParameterValuePtr(ParamCacheClusterId)
	if err != nil {
		return nil, err
	}

	input.AutoMinorVersionUpgrade, err = s.GetParameterValueBoolPtr(ParamAutoMinorVersionUpgrade)
	if err != nil {
		return nil, err
	}

	input.CacheNodeType, err = s.GetParameterValuePtr(ParamCacheNodeType)
	if err != nil {
		return nil, err
	}

	input.CacheParameterGroupName, err = s.GetParameterValuePtr(ParamCacheParameterGroupName)
	if err != nil {
		return nil, err
	}

	input.CacheSubnetGroupName, err = s.GetParameterValuePtr(ParamCacheSubnetGroupName)
	if err != nil {
		return nil, err
	}

	input.Engine, err = s.GetParameterValuePtr(ParamEngine)
	if err != nil {
		return nil, err
	}

	input.EngineVersion, err = s.GetParameterValuePtr(ParamEngineVersion)
	if err != nil {
		return nil, err
	}

	input.NetworkType = elasticachetypes.NetworkTypeIpv4

	input.NumCacheNodes, err = s.GetParameterValueInt32Ptr(ParamNumCacheNodes)
	if err != nil {
		return nil, err
	}

	input.Port, err = s.GetParameterValueInt32Ptr(ParamPort)
	if err != nil {
		return nil, err
	}

	input.SecurityGroupIds, err = s.GetParameterValueStringArray(ParamSecurityGroupIds)
	if err != nil {
		return nil, err
	}

	input.TransitEncryptionEnabled, err = s.GetParameterValueBoolPtr(ParamTransitEncryptionEnabled)
	if err != nil {
		return nil, err
	}

	return input, nil
}

func (s *StateData) GenerateModifyCacheClusterInput(changedParams []string, cacheClusterInfo *elasticachetypes.CacheCluster) (*elasticache.ModifyCacheClusterInput, error) {
	modifiedState := NewEmptyState()
	updateMetadata := GetParametersMetaDataForCacheClusterUpdate()
	for k, m := range updateMetadata {
		if m.Required != nil && *m.Required {
			param := s.GetParameter(k)
			if param == nil {
				return nil, fmt.Errorf("required param '%s' not found in the state", k)
			}
			modifiedState.AddOrUpdateParameter(*param)
		} else if provisioner.TargetExistsInStringArray(changedParams, k) {
			param := s.GetParameter(k)
			if param == nil {
				return nil, fmt.Errorf("changed param '%s' not found in the state", k)
			}
			modifiedState.AddOrUpdateParameter(*param)
		}
	}

	input := &elasticache.ModifyCacheClusterInput{}

	var err error
	input.CacheClusterId, err = modifiedState.GetParameterValuePtr(ParamCacheClusterId)
	if err != nil {
		return nil, err
	}

	input.ApplyImmediately, err = modifiedState.GetParameterValueBoolPtr(ParamApplyImmediately)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.AutoMinorVersionUpgrade, err = modifiedState.GetParameterValueBoolPtr(ParamAutoMinorVersionUpgrade)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.CacheNodeType, err = modifiedState.GetParameterValuePtr(ParamCacheNodeType)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.CacheParameterGroupName, err = modifiedState.GetParameterValuePtr(ParamCacheParameterGroupName)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.EngineVersion, err = modifiedState.GetParameterValuePtr(ParamEngineVersion)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	input.NumCacheNodes, err = modifiedState.GetParameterValueInt32Ptr(ParamNumCacheNodes)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	if input.NumCacheNodes != nil {
		if *cacheClusterInfo.NumCacheNodes > *input.NumCacheNodes {
			// sort by created time, node created before should be at front
			sort.Slice(cacheClusterInfo.CacheNodes, func(i, j int) bool {
				if cacheClusterInfo.CacheNodes[i].CacheNodeCreateTime == nil {
					return false
				}
				if cacheClusterInfo.CacheNodes[j].CacheNodeCreateTime == nil {
					return true
				}

				return cacheClusterInfo.CacheNodes[i].CacheNodeCreateTime.Before(*cacheClusterInfo.CacheNodes[j].CacheNodeCreateTime)
			})

			for i := *input.NumCacheNodes; i < *cacheClusterInfo.NumCacheNodes; i++ {
				input.CacheNodeIdsToRemove = append(input.CacheNodeIdsToRemove, *cacheClusterInfo.CacheNodes[i].CacheNodeId)
			}
		}
	}

	input.SecurityGroupIds, err = modifiedState.GetParameterValueStringArray(ParamSecurityGroupIds)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return nil, err
	}

	return input, nil
}

func (s *StateData) ImportCacheCluster(r *elasticachetypes.CacheCluster) error {
	m := GetParametersMetaDataForCacheClusterImport()

	err := s.AddParameterForImportByValuePtr(ParamCacheClusterId, r.CacheClusterId, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamAutoMinorVersionUpgrade, r.AutoMinorVersionUpgrade, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamCacheNodeType, r.CacheNodeType, m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamEngine, "memcached", m)
	if err != nil {
		return err
	}

	err = s.AddParameterForImportByValuePtr(ParamTransitEncryptionEnabled, r.TransitEncryptionEnabled, m)
	if err != nil {
		return err
	}

	return nil
}
