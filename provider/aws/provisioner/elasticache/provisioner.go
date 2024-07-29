package elasticache

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/convox/convox/provider/aws/provisioner"
	"github.com/convox/logger"
)

const ProvisionerName = "convox-elasticache"

type Provisioner struct {
	elasticacheClient *elasticache.Client
	ec2Client         *ec2.Client
	storage           provisioner.Storage
	logger            *logger.Logger
}

func NewProvisioner(s provisioner.Storage) *Provisioner {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	return &Provisioner{
		elasticacheClient: elasticache.NewFromConfig(cfg),
		ec2Client:         ec2.NewFromConfig(cfg),
		storage:           s,
		logger:            logger.New("elastic-cache-provisioner"),
	}
}

func (p *Provisioner) Provision(id string, options map[string]string) error {
	if options[ParamEngine] == "redis" {
		if v, ok := options[ParamImport]; ok {
			p.logger.Logf("Start redis replication group import")
			if err := p.ImportReplicatonGroup(id, v, options); err != nil {
				return fmt.Errorf("failed to import redis replication group: %s", err)
			}
			return nil
		}

		_, err := p.storage.GetState(id)
		if err != nil {
			if provisioner.IsNotFoundError(err) {

				p.logger.Logf("Start provisioning for redis replication group")
				if err := p.InstallReplicationGroup(id, options); err != nil {
					return fmt.Errorf("failed to install redis replication group: %s", err)
				}
				return nil
			}
			return err
		}

		p.logger.Logf("Start redis replication group update")
		if err := p.UpdateReplicationGroup(id, options); err != nil {
			return fmt.Errorf("failed to update redis replication group: %s", err)
		}
		return nil
	} else if options[ParamEngine] == "memcached" {
		if v, ok := options[ParamImport]; ok {
			p.logger.Logf("Start memcached cache cluster import")
			if err := p.ImportCacheCluster(id, v, options); err != nil {
				return fmt.Errorf("failed to import memcached cache cluster: %s", err)
			}
			return nil
		}

		_, err := p.storage.GetState(id)
		if err != nil {
			if provisioner.IsNotFoundError(err) {

				p.logger.Logf("Start provisioning for memcached cache cluster")
				if err := p.InstallCacheCluster(id, options); err != nil {
					return fmt.Errorf("failed to install memcache cache cluster: %s", err)
				}
				return nil
			}
			return err
		}

		p.logger.Logf("Start memcached cache cluster update")
		if err := p.UpdateCacheCluster(id, options); err != nil {
			return fmt.Errorf("failed to update memcached cache cluster: %s", err)
		}
		return nil
	}

	return fmt.Errorf("%s not supported", options[ParamEngine])
}

func (p *Provisioner) InstallReplicationGroup(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("already found saved state for this id: %s", id)
	}

	options[ParamReplicationGroupId] = provisioner.GenShortResourceName(id)

	if err := p.ApplyReplicationGroupInstallDefaults(options); err != nil {
		return err
	}

	installParamsMeta := GetParametersMetaDataForReplicationGroupInstall()

	for k := range options {
		if _, has := installParamsMeta[k]; !has {
			return fmt.Errorf("param '%s' is not allowed or supported", k)
		}
	}

	params := []Parameter{}
	for p, m := range installParamsMeta {
		newParam := NewParameter(p, options[p], m)
		if err := newParam.Validate(); err != nil {
			return err
		}

		params = append(params, *newParam)
	}

	for i := range params {
		if err := params[i].Validate(); err != nil {
			return err
		}
	}

	p.logger.Logf("Generating the state data for id: %s", id)
	stateData := NewState(id, StateProvisioning, params)

	if err := p.createSecurityGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create security group: %s", err)
	}

	if err := p.createCacheSubnetGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create subnet group: %s", err)
	}

	createOptions, err := stateData.GenerateCreateReplicationGroupInput()
	if err != nil {
		return err
	}

	createOptions.Tags = []elasticachetypes.Tag{
		{
			Key:   aws.String(ProvisionerName),
			Value: aws.String(stateData.Id),
		},
	}

	p.logger.Logf("Installing redis replication group: %s", id)
	_, err = p.elasticacheClient.CreateReplicationGroup(context.TODO(), createOptions)
	if err != nil {
		p.logger.Errorf("Failed to create redis replication group: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.UninstallReplicationGroup(id)
		return err
	}

	if err := p.SaveState(id, stateData); err != nil {
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.UninstallReplicationGroup(id)
		return err
	}

	p.logger.Logf("Successfully installed redis replication group, it may take some times to be available")
	p.storage.SendStateLog(id, "successfully installed redis replication group")
	return nil
}

func (p *Provisioner) InstallCacheCluster(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("already found saved state for this id: %s", id)
	}

	options[ParamCacheClusterId] = provisioner.GenShortResourceName(id)

	if err := p.ApplyCacheClusterInstallDefaults(options); err != nil {
		return err
	}

	installParamsMeta := GetParametersMetaDataForCacheClusterInstall()

	for k := range options {
		if _, has := installParamsMeta[k]; !has {
			return fmt.Errorf("param '%s' is not allowed or supported", k)
		}
	}

	params := []Parameter{}
	for p, m := range installParamsMeta {
		newParam := NewParameter(p, options[p], m)
		if err := newParam.Validate(); err != nil {
			return err
		}

		params = append(params, *newParam)
	}

	for i := range params {
		if err := params[i].Validate(); err != nil {
			return err
		}
	}

	p.logger.Logf("Generating the state data for id: %s", id)
	stateData := NewState(id, StateProvisioning, params)

	if err := p.createSecurityGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create security group: %s", err)
	}

	if err := p.createCacheSubnetGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create subnet group: %s", err)
	}

	createOptions, err := stateData.GenerateCreateCacheClusterInput()
	if err != nil {
		return err
	}

	createOptions.Tags = []elasticachetypes.Tag{
		{
			Key:   aws.String(ProvisionerName),
			Value: aws.String(stateData.Id),
		},
	}

	p.logger.Logf("Installing memcached cache cluster: %s", id)
	_, err = p.elasticacheClient.CreateCacheCluster(context.TODO(), createOptions)
	if err != nil {
		p.logger.Errorf("Failed to create cache cluster: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.UninstallCacheCluster(id)
		return err
	}

	if err := p.SaveState(id, stateData); err != nil {
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.UninstallCacheCluster(id)
		return err
	}

	p.logger.Logf("Successfully installed memcached cache cluster, it may take some times to be available")
	p.storage.SendStateLog(id, "successfully installed memcached cache cluster")
	return nil
}

func (p *Provisioner) UpdateReplicationGroup(id string, optoins map[string]string) error {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return err
	}

	updateMetadata := GetParametersMetaDataForReplicationGroupUpdate()

	changedParams := []string{}
	for k, v := range optoins {
		changed, err := stateData.UpdateParameterValueForCacheUpdate(k, v, updateMetadata)
		if err != nil {
			return fmt.Errorf("failed to update parameter value: %s", err)
		}
		if changed {
			changedParams = append(changedParams, k)
		}
	}

	if len(changedParams) == 0 {
		p.logger.Logf("no changes detected")
		p.storage.SendStateLog(id, "no changes detected")
		return nil
	}

	noModificationExceptReplicaCount := false
	if provisioner.TargetExistsInStringArray(changedParams, ParamNumCacheClusters) {
		repGrpId, err := stateData.GetParameterValue(ParamReplicationGroupId)
		if err != nil {
			return err
		}

		rp, err := p.GetReplicationGroup(repGrpId)
		if err != nil {
			return err
		}

		if len(rp.NodeGroups) == 0 {
			return fmt.Errorf("node groups is empty")
		}

		curCnt := int32(len(rp.NodeGroups[0].NodeGroupMembers))

		replica, err := stateData.GetParameterValueInt32Ptr(ParamNumCacheClusters)
		if err != nil {
			return err
		}
		if replica == nil {
			return fmt.Errorf("no value found for parameter nodes")
		}

		p.logger.Logf("Updating replication group: %s", id)

		if curCnt < *replica {
			_, err = p.elasticacheClient.IncreaseReplicaCount(context.Background(), &elasticache.IncreaseReplicaCountInput{
				ReplicationGroupId: aws.String(repGrpId),
				NewReplicaCount:    aws.Int32(*replica - 1),
				ApplyImmediately:   aws.Bool(true),
			})
			if err != nil {
				return fmt.Errorf("failed to increase replica count: %s", err)
			}
		} else if curCnt > *replica {
			_, err = p.elasticacheClient.DecreaseReplicaCount(context.Background(), &elasticache.DecreaseReplicaCountInput{
				ReplicationGroupId: aws.String(repGrpId),
				NewReplicaCount:    aws.Int32(*replica - 1),
				ApplyImmediately:   aws.Bool(true),
			})
			if err != nil {
				return fmt.Errorf("failed to decrease replica count: %s", err)
			}
		}

		noModificationExceptReplicaCount = len(changedParams) == 1
	}

	p.logger.Logf("found changes in these following parameters: %s", strings.Join(changedParams, ", "))

	if !noModificationExceptReplicaCount {
		modifyReqInput, err := stateData.GenerateModifyReplicationGroupInput(changedParams)
		if err != nil {
			return fmt.Errorf("failed to generate modification config: %s", err)
		}

		// TODO: Add option to manage this from user side
		modifyReqInput.ApplyImmediately = aws.Bool(true)

		p.logger.Logf("Updating replication group: %s", id)

		_, err = p.elasticacheClient.ModifyReplicationGroup(context.Background(), modifyReqInput)
		if err != nil {
			return fmt.Errorf("modification request failed with an error: %s", err)
		}
	}

	if err := p.SaveState(id, stateData); err != nil {
		return err
	}

	p.logger.Logf("Successfully applied the replication group updates")
	p.storage.SendStateLog(id, "successfully applied the replication group updates")
	return nil
}

func (p *Provisioner) UpdateCacheCluster(id string, optoins map[string]string) error {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return err
	}

	updateMetadata := GetParametersMetaDataForCacheClusterUpdate()

	changedParams := []string{}
	for k, v := range optoins {
		changed, err := stateData.UpdateParameterValueForCacheUpdate(k, v, updateMetadata)
		if err != nil {
			return fmt.Errorf("failed to update parameter value: %s", err)
		}
		if changed {
			changedParams = append(changedParams, k)
		}
	}

	if len(changedParams) == 0 {
		p.logger.Logf("no changes detected")
		p.storage.SendStateLog(id, "no changes detected")
		return nil
	}
	p.logger.Logf("found changes in these following parameters: %s", strings.Join(changedParams, ", "))

	clusterId, err := stateData.GetParameterValue(ParamCacheClusterId)
	if err != nil {
		return err
	}

	cacheClusterInfo, err := p.GetCacheCluster(clusterId)
	if err != nil {
		return err
	}

	modifyReqInput, err := stateData.GenerateModifyCacheClusterInput(changedParams, cacheClusterInfo)
	if err != nil {
		return fmt.Errorf("failed to generate modification config: %s", err)
	}

	// TODO: Add option to manage this from user side
	modifyReqInput.ApplyImmediately = aws.Bool(true)

	p.logger.Logf("Updating cache cluster: %s", id)

	_, err = p.elasticacheClient.ModifyCacheCluster(context.Background(), modifyReqInput)
	if err != nil {
		return fmt.Errorf("modification request failed with an error: %s", err)
	}

	if err := p.SaveState(id, stateData); err != nil {
		return err
	}

	p.logger.Logf("Successfully applied the cache cluster updates")
	p.storage.SendStateLog(id, "successfully applied the cache cluster updates")
	return nil
}

func (p *Provisioner) ImportReplicatonGroup(id string, repGrpId string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return err
	}

	state := NewStateForImport(id)

	p.logger.Logf("Fetching redis replication group details: %s", repGrpId)

	rgrp, err := p.GetReplicationGroup(repGrpId)
	if err != nil {
		return err
	}

	var authToken *string
	if options[ParamAuthToken] != "" {
		authToken = aws.String(options[ParamAuthToken])
	}

	if err := state.ImportReplicationGroup(rgrp, authToken); err != nil {
		return err
	}

	if err := p.SaveState(id, state); err != nil {
		return err
	}

	p.logger.Logf("Successfully imported replication group")
	p.storage.SendStateLog(id, "successfully imported replication group")
	return nil
}

func (p *Provisioner) ImportCacheCluster(id string, clusterId string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return err
	}

	state := NewStateForImport(id)

	p.logger.Logf("Fetching memcached cache cluster details: %s", clusterId)

	cluster, err := p.GetCacheCluster(clusterId)
	if err != nil {
		return err
	}

	if err := state.ImportCacheCluster(cluster); err != nil {
		return err
	}

	if err := p.SaveState(id, state); err != nil {
		return err
	}

	p.logger.Logf("Successfully imported cache cluster")
	p.storage.SendStateLog(id, "successfully imported cache cluster")
	return nil
}

func (p *Provisioner) Uninstall(id string) error {
	p.logger.Logf("Fetching the state data for id: %s", id)
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	p.logger.Logf("Loading the state data for id: %s", id)
	state := NewEmptyState()
	if err := state.LoadState(stateBytes); err != nil {
		return err
	}

	delParam := state.GetParameter(ParamDeletionProtection)
	if delParam != nil && !delParam.IsValueEmpty() && strings.EqualFold(*delParam.Value, "true") {
		p.logger.Logf("deletion protection is enabled, skipping uninstallation")
		return nil
	}

	engine, err := state.GetParameterValue(ParamEngine)
	if err != nil {
		return err
	}

	switch engine {
	case "redis":
		return p.UninstallReplicationGroup(id)
	case "memcached":
		return p.UninstallCacheCluster(id)
	default:
		return fmt.Errorf("invalid cache engine: %s", engine)
	}
}

func (p *Provisioner) UninstallReplicationGroup(id string) error {
	p.logger.Logf("Fetching the state data for id: %s", id)
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	p.logger.Logf("Loading the state data for id: %s", id)
	state := NewEmptyState()
	if err := state.LoadState(stateBytes); err != nil {
		return err
	}

	delParam := state.GetParameter(ParamDeletionProtection)
	if delParam != nil && !delParam.IsValueEmpty() && strings.EqualFold(*delParam.Value, "true") {
		p.logger.Logf("deletion protection is enabled, skipping uninstallation")
		return nil
	}

	if err := p.deleteReplicationGroupIfManaged(state); err != nil {
		return err
	}

	if err := p.deleteSecurityGroupIfManaged(state); err != nil {
		return err
	}

	if err := p.deleteCacheSubnetGroupIfManaged(state); err != nil {
		return err
	}

	p.logger.Logf("Uninstalled the redis resources for id: %s", id)

	return nil
}

func (p *Provisioner) UninstallCacheCluster(id string) error {
	p.logger.Logf("Fetching the state data for id: %s", id)
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	p.logger.Logf("Loading the state data for id: %s", id)
	state := NewEmptyState()
	if err := state.LoadState(stateBytes); err != nil {
		return err
	}

	delParam := state.GetParameter(ParamDeletionProtection)
	if delParam != nil && !delParam.IsValueEmpty() && strings.EqualFold(*delParam.Value, "true") {
		p.logger.Logf("deletion protection is enabled, skipping uninstallation")
		return nil
	}

	if err := p.deleteCacheClusterIfManaged(state); err != nil {
		return err
	}

	if err := p.deleteSecurityGroupIfManaged(state); err != nil {
		return err
	}

	if err := p.deleteCacheSubnetGroupIfManaged(state); err != nil {
		return err
	}

	p.logger.Logf("Uninstalled the memcahed resources for id: %s", id)

	return nil
}

func (p *Provisioner) SaveState(id string, stateData *StateData) error {
	p.logger.Logf("Saving the state data for id: %s", id)
	stateBytes, err := stateData.GetStateInBytes()
	if err != nil {
		p.logger.Errorf("Failed to get state bytes: %s", err)
		return err
	}

	if err := p.storage.SaveState(id, stateBytes, ProvisionerName); err != nil {
		p.logger.Errorf("Failed to save state: %s", err)
		return err
	}
	return nil
}

func (p *Provisioner) WaitUntilReplicationGroupIsAvailable(id string) error {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return err
	}

	repGrpId, err := stateData.GetParameterValue(ParamReplicationGroupId)
	if err != nil {
		return err
	}

	p.logger.Logf("Waiting for redis replication group to be available")
	if err := p.waitUntilTargetReplicationGroupIsAvailable(repGrpId); err != nil {
		return err
	}
	p.logger.Logf("Redis replicaiton group is now available")
	return nil
}

func (p *Provisioner) WaitUntilCacheClusterIsAvailable(id string) error {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return err
	}

	clusterId, err := stateData.GetParameterValue(ParamCacheClusterId)
	if err != nil {
		return err
	}

	p.logger.Logf("Waiting for memcached cache cluster to be available")
	if err := p.waitUntilTargetCacheClusterIsAvailable(clusterId); err != nil {
		return err
	}
	p.logger.Logf("Memcached cache cluster is now available")
	return nil
}

func (p *Provisioner) GetConnectionInfo(id string) (*provisioner.ConnectionInfo, error) {
	p.logger.Logf("Fetching the state data for id: %s", id)
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return nil, err
	}

	p.logger.Logf("Loading the state data for id: %s", id)
	state := NewEmptyState()
	if err := state.LoadState(stateBytes); err != nil {
		return nil, err
	}

	engine, err := state.GetParameterValue(ParamEngine)
	if err != nil {
		return nil, err
	}

	switch engine {
	case "redis":
		return p.GetReplicationGroupConnectionInfo(id)
	case "memcached":
		return p.GetCacheClusterConnectionInfo(id)
	default:
		return nil, fmt.Errorf("invalid cache engine: %s", engine)
	}
}

func (p *Provisioner) GetReplicationGroupConnectionInfo(id string) (*provisioner.ConnectionInfo, error) {
	p.logger.Logf("Fetching the state data for id: %s", id)
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return nil, err
	}

	p.logger.Logf("Loading the state data for id: %s", id)
	state := NewEmptyState()
	if err := state.LoadState(stateBytes); err != nil {
		return nil, err
	}

	var (
		authToken string
		port      string
	)

	pauthTokenPtr := state.GetParameter(ParamAuthToken)
	if pauthTokenPtr != nil && !pauthTokenPtr.IsValueEmpty() {
		authToken = *pauthTokenPtr.Value
	}

	portPtr := state.GetParameter(ParamPort)
	if portPtr != nil && !portPtr.IsValueEmpty() {
		port = *portPtr.Value
	}

	if state.Host == "" || portPtr == nil {
		if err := p.WaitUntilReplicationGroupIsAvailable(id); err != nil {
			return nil, err
		}

		repGrpId, err := state.GetParameterValue(ParamReplicationGroupId)
		if err != nil {
			return nil, err
		}

		rResp, err := p.GetReplicationGroup(repGrpId)
		if err != nil {
			return nil, err
		}

		if len(rResp.NodeGroups) == 0 {
			return nil, fmt.Errorf("redis replicaiton group's node group is empty")
		}

		pEndpoint := rResp.NodeGroups[0].PrimaryEndpoint

		if pEndpoint == nil || pEndpoint.Address == nil || pEndpoint.Port == nil {
			return nil, fmt.Errorf("redis replication group primary endpoint is not found")
		}

		state.Host = *pEndpoint.Address

		port = strconv.FormatInt(int64(*pEndpoint.Port), 10)

		state.AddOrUpdateParameter(*NewParameter(ParamPort, port, &MetaData{}))

		if err := p.SaveState(id, state); err != nil {
			return nil, err
		}
	}

	return &provisioner.ConnectionInfo{
		Host:     state.Host,
		Port:     port,
		Password: authToken,
	}, nil
}

func (p *Provisioner) GetCacheClusterConnectionInfo(id string) (*provisioner.ConnectionInfo, error) {
	p.logger.Logf("Fetching the state data for id: %s", id)
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return nil, err
	}

	p.logger.Logf("Loading the state data for id: %s", id)
	state := NewEmptyState()
	if err := state.LoadState(stateBytes); err != nil {
		return nil, err
	}

	var port string

	portPtr := state.GetParameter(ParamPort)
	if portPtr != nil && !portPtr.IsValueEmpty() {
		port = *portPtr.Value
	}

	if state.Host == "" || portPtr == nil {
		if err := p.WaitUntilCacheClusterIsAvailable(id); err != nil {
			return nil, err
		}

		clusterId, err := state.GetParameterValue(ParamCacheClusterId)
		if err != nil {
			return nil, err
		}

		cResp, err := p.GetCacheCluster(clusterId)
		if err != nil {
			return nil, err
		}

		pEndpoint := cResp.ConfigurationEndpoint

		if pEndpoint == nil || pEndpoint.Address == nil || pEndpoint.Port == nil {
			return nil, fmt.Errorf("memcached endpoint is not found")
		}

		state.Host = *pEndpoint.Address

		port = strconv.FormatInt(int64(*pEndpoint.Port), 10)

		state.AddOrUpdateParameter(*NewParameter(ParamPort, port, &MetaData{}))

		if err := p.SaveState(id, state); err != nil {
			return nil, err
		}
	}

	return &provisioner.ConnectionInfo{
		Host: state.Host,
		Port: port,
	}, nil
}

func (p *Provisioner) createSecurityGroupIfNotProvided(state *StateData) error {
	securityGroups, err := state.GetParameterValue(ParamSecurityGroupIds)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return err
	}
	if securityGroups != "" {
		p.logger.Logf("vpc security group id is provided, using it")
		return nil
	}

	// check if we already have managed sg, incase of create failure of the redis
	p.logger.Logf("checking if vpc security group id is already created")
	sgList, err := p.GetSecurityGroupByTags(map[string]string{
		ProvisionerName: state.Id,
	})
	if err != nil {
		return err
	}

	if len(sgList) > 0 {
		p.logger.Logf("found vpc security group id: %s", *sgList[0].GroupId)
		return state.InitializeParameterValue(ParamSecurityGroupIds, *sgList[0].GroupId)
	}

	// otherwise create one
	p.logger.Logf("creating vpc security group id")
	vpcID, err := state.GetParameterValue(ParamVPC)
	if err != nil {
		return err
	}

	vpcCidr, err := p.GetVPCCIDR(vpcID)
	if err != nil {
		return err
	}

	port, err := state.GetParameterValueInt32Ptr(ParamPort)
	if err != nil {
		return err
	}
	if port == nil {
		return fmt.Errorf("port parameter is not defined")
	}

	groupID, err := p.CreateSecurityGroup(fmt.Sprintf("cache-sg-%s", state.Id), vpcID, state.Id)
	if err != nil {
		return fmt.Errorf("failed to create cache security group: %s", err)
	}

	if err := p.AddIngressRule(groupID, *port, vpcCidr); err != nil {
		p.DeleteSecurityGroup(groupID)
		return err
	}

	if err := state.InitializeParameterValue(ParamSecurityGroupIds, groupID); err != nil {
		p.DeleteSecurityGroup(groupID)
		return err
	}

	return nil
}

func (p *Provisioner) createCacheSubnetGroupIfNotProvided(state *StateData) error {
	subnetGroupName, err := state.GetParameterValue(ParamCacheSubnetGroupName)
	if err != nil && !provisioner.IsNotFoundError(err) {
		return err
	}
	if subnetGroupName != "" {
		p.logger.Logf("cache subnet group id is provided, using it")
		return nil
	}

	p.logger.Logf("checking if cacher subnet group is already created")
	subnetGroupName = fmt.Sprintf("cache-subg-%s", state.Id)
	sbg, err := p.GetCacheSubnetGroup(subnetGroupName)
	if sbg != nil {
		if err := state.InitializeParameterValue(ParamCacheSubnetGroupName, subnetGroupName); err != nil {
			return err
		}
		return nil
	}

	p.logger.Logf("creating cache subnet group")

	subnetIdsStr, err := state.GetParameterValue(ParamSubnetIds)
	if err != nil {
		return err
	}
	groupName, err := p.CreateCacheSubnetGroup(subnetGroupName, provisioner.ConvertToStringArray(subnetIdsStr), state.Id)
	if err != nil {
		return err
	}

	if err := state.InitializeParameterValue(ParamCacheSubnetGroupName, groupName); err != nil {
		p.DeleteCacheSubnetGroup(subnetGroupName)
		return err
	}

	return nil
}

func (p *Provisioner) GetReplicationGroup(identifier string) (*elasticachetypes.ReplicationGroup, error) {
	input := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(identifier),
	}

	result, err := p.elasticacheClient.DescribeReplicationGroups(context.Background(), input)
	if err != nil {
		if err, ok := err.(awserr.Error); ok && strings.Contains(err.Code(), "ReplicationGroupNotFound") {
			return nil, fmt.Errorf("replication group not found")
		}
		return nil, fmt.Errorf("failed to describe replication group: %s", err)
	}

	if len(result.ReplicationGroups) == 0 {
		return nil, fmt.Errorf("replication group not found")
	}

	return &result.ReplicationGroups[0], nil
}

func (p *Provisioner) GetCacheCluster(clusterId string) (*elasticachetypes.CacheCluster, error) {
	input := &elasticache.DescribeCacheClustersInput{
		CacheClusterId:                          aws.String(clusterId),
		ShowCacheClustersNotInReplicationGroups: aws.Bool(true),
		ShowCacheNodeInfo:                       aws.Bool(true),
	}

	result, err := p.elasticacheClient.DescribeCacheClusters(context.Background(), input)
	if err != nil {
		if err, ok := err.(awserr.Error); ok && strings.Contains(err.Code(), "CacheClusterNotFound") {
			return nil, fmt.Errorf("cache cluster not found")
		}
		return nil, fmt.Errorf("failed to describe cache cluster: %s", err)
	}

	if len(result.CacheClusters) == 0 {
		return nil, fmt.Errorf("cache cluster not found")
	}

	return &result.CacheClusters[0], nil
}

func (p *Provisioner) DeleteReplicationGroup(identifier string) error {
	input := &elasticache.DeleteReplicationGroupInput{
		ReplicationGroupId:      aws.String(identifier),
		FinalSnapshotIdentifier: aws.String(fmt.Sprintf("%s-final-snapshot-%d", identifier, time.Now().UTC().Unix())),
	}

	_, err := p.elasticacheClient.DeleteReplicationGroup(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to delete replication group: %s", err)
	}

	return nil
}

func (p *Provisioner) DeleteCacheCluster(identifier string) error {
	input := &elasticache.DeleteCacheClusterInput{
		CacheClusterId: aws.String(identifier),
	}

	_, err := p.elasticacheClient.DeleteCacheCluster(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to delete cache cluster: %s", err)
	}

	return nil
}

func (p *Provisioner) deleteReplicationGroupIfManaged(state *StateData) error {
	replciatonGroupId, err := state.GetParameterValue(ParamReplicationGroupId)
	if err != nil {
		return err
	}

	resp, err := p.GetReplicationGroup(replciatonGroupId)
	if err != nil {
		if provisioner.IsNotFoundError(err) {
			return nil
		}
		return err
	}

	tagResp, err := p.elasticacheClient.ListTagsForResource(context.Background(), &elasticache.ListTagsForResourceInput{
		ResourceName: resp.ARN,
	})
	if err != nil {
		return err
	}

	for _, v := range tagResp.TagList {
		if v.Key != nil && *v.Key == ProvisionerName && v.Value != nil && *v.Value == state.Id {
			return p.DeleteReplicationGroup(replciatonGroupId)
		}
	}
	return nil
}

func (p *Provisioner) deleteCacheClusterIfManaged(state *StateData) error {
	clusterId, err := state.GetParameterValue(ParamCacheClusterId)
	if err != nil {
		return err
	}

	resp, err := p.GetCacheCluster(clusterId)
	if err != nil {
		if provisioner.IsNotFoundError(err) {
			return nil
		}
		return err
	}

	tagResp, err := p.elasticacheClient.ListTagsForResource(context.Background(), &elasticache.ListTagsForResourceInput{
		ResourceName: resp.ARN,
	})
	if err != nil {
		return err
	}

	for _, v := range tagResp.TagList {
		if v.Key != nil && *v.Key == ProvisionerName && v.Value != nil && *v.Value == state.Id {
			return p.DeleteCacheCluster(clusterId)
		}
	}
	return nil
}

func (p *Provisioner) deleteSecurityGroupIfManaged(state *StateData) error {
	sgList, err := p.GetSecurityGroupByTags(map[string]string{
		ProvisionerName: state.Id,
	})
	if err != nil {
		return err
	}

	for i := range sgList {
		p.logger.Logf("Deleting cache security group: %s", *sgList[i].GroupId)
		p.DeleteSecurityGroup(*sgList[i].GroupId)
	}
	return nil
}

func (p *Provisioner) deleteCacheSubnetGroupIfManaged(state *StateData) error {
	subnetGroup, err := state.GetParameterValuePtr(ParamCacheSubnetGroupName)
	if err != nil {
		return err
	}

	if subnetGroup == nil {
		return nil
	}

	sbg, err := p.GetCacheSubnetGroup(*subnetGroup)
	if err != nil {
		if provisioner.IsNotFoundError(err) {
			return nil
		}
		return err
	}

	p.logger.Logf("Deleting cache subnet group: %s", *sbg.CacheSubnetGroupName)
	p.DeleteCacheSubnetGroup(*sbg.CacheSubnetGroupName)
	return nil
}
