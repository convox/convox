package rds

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/convox/convox/provider/aws/provisioner"
	"github.com/convox/logger"
)

const ProvisionerName = "convox-rds"

type Provisioner struct {
	rdsClient *rds.Client
	ec2Client *ec2.Client
	storage   provisioner.Storage
	logger    *logger.Logger
}

func NewProvisioner(s provisioner.Storage) *Provisioner {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	return &Provisioner{
		rdsClient: rds.NewFromConfig(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		storage:   s,
		logger:    logger.New("rds-provisioner"),
	}
}

func (p *Provisioner) Provision(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil {
		if IsNotFoundError(err) {
			return p.Install(id, options)
		}
		return err
	}

	if ok, v := IsDbImport(options); ok {
		pass := options[ParamMasterUserPassword]
		if pass == "" {
			return fmt.Errorf("db password is required for import")
		}
		return p.Import(id, v, &pass)
	}

	return p.Update(id, options)
}

func (p *Provisioner) Install(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !IsNotFoundError(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("already found saved state for this id: %s", id)
	}

	options[ParamDBInstanceIdentifier] = id

	if _, has := options[ParamPort]; !has {
		options[ParamPort] = DefaultDbPort(options[ParamEngine])
	}

	if _, has := options[ParamMasterUserPassword]; !has {
		options[ParamMasterUserPassword], err = GenerateSecurePassword(36)
		if err != nil {
			return fmt.Errorf("failed to generate password: %s", err)
		}
	}

	allParamNames := ParametersNameList()
	params := []Parameter{}
	for _, p := range allParamNames {
		m, err := ParameterMetaDataForInstall(p)
		if err != nil {
			return err
		}

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
		return err
	}

	if err := p.createDBSubnetGroupIfNotProvided(stateData); err != nil {
		return err
	}

	createOptions, err := stateData.GenerateCreateDBInstanceInput()
	if err != nil {
		return err
	}

	createOptions.Tags = []rdstypes.Tag{
		{
			Key:   aws.String(ProvisionerName),
			Value: aws.String(stateData.Id),
		},
	}

	p.logger.Logf("Installing db instance: %s", id)
	dbCreateResp, err := p.rdsClient.CreateDBInstance(context.TODO(), createOptions)
	if err != nil {
		p.logger.Errorf("Failed to create db instance: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	stateData.Host = *dbCreateResp.DBInstance.Endpoint.Address

	p.logger.Logf("Saving the state data for id: %s", id)
	stateBytes, err := stateData.GetStateInBytes()
	if err != nil {
		p.logger.Errorf("Failed to get state bytes: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	if err := p.storage.SaveState(id, stateBytes); err != nil {
		p.logger.Errorf("Failed to save state: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}
	p.logger.Logf("Successfully installed db instance, it may take some times to be available")
	return nil
}

func (p *Provisioner) Update(id string, optoins map[string]string) error {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return err
	}

	changedParams := []string{}
	for k, v := range optoins {
		changed, err := stateData.UpdateParameterValue(k, v)
		if err != nil {
			return err
		}
		if changed {
			changedParams = append(changedParams, k)
		}
	}

	if len(changedParams) == 0 {
		p.logger.Logf("no changes detected")
		return nil
	}

	// handle promote read replica seperatly

	modifyReqInput, err := stateData.GenerateModifyDBInstanceInput(changedParams)
	if err != nil {
		return err
	}

	// TODO: Add option manage this from user side
	modifyReqInput.ApplyImmediately = aws.Bool(true)

	p.logger.Logf("Updating db instance: %s", id)

	_, err = p.rdsClient.ModifyDBInstance(context.Background(), modifyReqInput)
	if err != nil {
		return err
	}

	p.logger.Logf("Saving the state data for id: %s", id)
	newstateBytes, err := stateData.GetStateInBytes()
	if err != nil {
		p.logger.Errorf("Failed to get state bytes: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	if err := p.storage.SaveState(id, newstateBytes); err != nil {
		p.logger.Errorf("Failed to save state: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	p.logger.Logf("Successfully applied the the updates")

	return nil
}

func (p *Provisioner) Import(id string, dbIdentifier string, pass *string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !IsNotFoundError(err) {
		return err
	}

	state := NewStateForImport(id)

	p.logger.Logf("Fetching db instance details: %s", dbIdentifier)

	db, err := p.GetDBInstance(dbIdentifier)
	if err != nil {
		return err
	}

	if err := state.ImportState(db, pass); err != nil {
		return err
	}

	p.logger.Logf("Saving the state data for id: %s", id)
	stateBytes, err := state.GetStateInBytes()
	if err != nil {
		p.logger.Errorf("Failed to get state bytes: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	if err := p.storage.SaveState(id, stateBytes); err != nil {
		p.logger.Errorf("Failed to save state: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

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

	if err := p.deleteDBInstancesIfManaged(state); err != nil {
		return err
	}

	if err := p.deleteSecurityGroupIfManaged(state); err != nil {
		return err
	}

	if err := p.deleteDBSubnetGroupIfManaged(state); err != nil {
		return err
	}

	p.logger.Logf("Uninstalled the db resources for id: %s", id)

	return nil
}

func (p *Provisioner) WaitUnitlAvailable(id string) error {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return err
	}

	dbIdentifier, err := stateData.GetParameterValue(ParamDBInstanceIdentifier)
	if err != nil {
		return err
	}

	p.logger.Logf("Waiting for db instance to be available")
	if err := p.waitUntilTargetDBIsAvailableAfterInstall(dbIdentifier); err != nil {
		return err
	}
	p.logger.Logf("Db instance is now available")
	return nil
}

func (p *Provisioner) IsDbAvailable(id string) (bool, error) {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return false, err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return false, err
	}

	dbIdentifier, err := stateData.GetParameterValue(ParamDBInstanceIdentifier)
	if err != nil {
		return false, err
	}

	status, err := p.getDbStatus(dbIdentifier)
	if err != nil {
		return false, err
	}

	return targetExistsInStringArray(DbInstanceAvailableStates, status), nil
}

func (p *Provisioner) GetConnectionInfo(id string) (*ConnectionInfo, error) {
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

	username, err := state.GetParameterValue(ParamMasterUsername)
	if err != nil {
		return nil, err
	}

	pass, err := state.GetParameterValue(ParamMasterUserPassword)
	if err != nil {
		return nil, err
	}

	port, err := state.GetParameterValue(ParamPort)
	if err != nil {
		return nil, err
	}

	dbName, err := state.GetParameterValue(ParamDBName)
	if err != nil {
		return nil, err
	}

	return &ConnectionInfo{
		Host:     state.Host,
		Port:     port,
		UserName: username,
		Password: pass,
		Database: dbName,
	}, nil
}

func (p *Provisioner) createSecurityGroupIfNotProvided(state *StateData) error {
	securityGroups, err := state.GetParameterValue(ParamVPCSecurityGroups)
	if err != nil && !IsNotFoundError(err) {
		return err
	}
	if securityGroups != "" {
		p.logger.Logf("vpc security group id is provided, using it")
		return nil
	}

	// check if we already have managed sg, incase of create failure of the db
	p.logger.Logf("checking if vpc security group id is already created")
	sgList, err := p.GetSecurityGroupByTags(map[string]string{
		ProvisionerName: state.Id,
	})
	if err != nil {
		return err
	}

	if len(sgList) > 0 {
		p.logger.Logf("found vpc security group id: %s", *sgList[0].GroupId)
		return state.InitializeParameterValue(ParamVPCSecurityGroups, *sgList[0].GroupId)
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

	groupID, err := p.CreateSecurityGroup(fmt.Sprintf("db-sg-%s", state.Id), vpcID, state.Id)
	if err != nil {
		return fmt.Errorf("failed to create db security group: %s", err)
	}

	if err := p.AddIngressRule(groupID, *port, vpcCidr); err != nil {
		p.DeleteSecurityGroup(groupID)
		return err
	}

	if err := state.InitializeParameterValue(ParamVPCSecurityGroups, groupID); err != nil {
		p.DeleteSecurityGroup(groupID)
		return err
	}

	return nil
}

func (p *Provisioner) createDBSubnetGroupIfNotProvided(state *StateData) error {
	subnetGroupName, err := state.GetParameterValue(ParamDBSubnetGroupName)
	if err != nil && !IsNotFoundError(err) {
		return err
	}
	if subnetGroupName != "" {
		p.logger.Logf("db subnet group id is provided, using it")
		return nil
	}

	p.logger.Logf("checking if db subnet group is already created")
	subnetGroupName = fmt.Sprintf("db-subg-%s", state.Id)
	dbSbg, err := p.GetDBSubnetGroup(*&subnetGroupName)
	if dbSbg != nil {
		if err := state.InitializeParameterValue(ParamDBSubnetGroupName, subnetGroupName); err != nil {
			return err
		}
		return nil
	}

	p.logger.Logf("creating db subnet group")

	subnetIdsStr, err := state.GetParameterValue(ParamSubnetIds)
	if err != nil {
		return err
	}
	groupName, err := p.CreateDBSubnetGroup(subnetGroupName, convertToStringArray(subnetIdsStr), state.Id)
	if err != nil {
		return fmt.Errorf("failed to create db subnet group: %s", err)
	}

	if err := state.InitializeParameterValue(ParamDBSubnetGroupName, groupName); err != nil {
		p.DeleteDBSubnetGroup(subnetGroupName)
		return err
	}

	return nil
}

func (p *Provisioner) GetDBInstance(dbIdentifier string) (*rdstypes.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(dbIdentifier),
	}

	result, err := p.rdsClient.DescribeDBInstances(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe db instances: %s", err)
	}

	if len(result.DBInstances) == 0 {
		return nil, fmt.Errorf("db instance not found")
	}

	return &result.DBInstances[0], nil
}

func (p *Provisioner) GetDBInstancesByTags(tags map[string]string) ([]rdstypes.DBInstance, error) {
	var filters []rdstypes.Filter
	for key, value := range tags {
		filters = append(filters, rdstypes.Filter{
			Name:   aws.String("tag:" + key),
			Values: []string{value},
		})
	}
	input := &rds.DescribeDBInstancesInput{
		Filters: filters,
	}

	result, err := p.rdsClient.DescribeDBInstances(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe db instances: %s", err)
	}

	return result.DBInstances, nil
}

func (p *Provisioner) DeleteDBInstance(dbIdentifier string) error {
	_, err := p.rdsClient.DeleteDBInstance(context.Background(), &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier:      aws.String(dbIdentifier),
		FinalDBSnapshotIdentifier: aws.String(fmt.Sprintf("%s-final-snapshot", dbIdentifier)),
	})
	if err != nil {
		return fmt.Errorf("failed to delete db instance: %s", err)
	}

	return nil
}

func (p *Provisioner) deleteDBInstancesIfManaged(state *StateData) error {
	dbList, err := p.GetDBInstancesByTags(map[string]string{
		ProvisionerName: state.Id,
	})
	if err != nil {
		return err
	}

	for i := range dbList {
		p.logger.Logf("Deleting db instance: %s", *dbList[i].DBInstanceIdentifier)
		p.DeleteDBInstance(*dbList[i].DBInstanceIdentifier)
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
		p.logger.Logf("Deleting db security group: %s", *sgList[i].GroupId)
		p.DeleteSecurityGroup(*sgList[i].GroupId)
	}
	return nil
}

func (p *Provisioner) deleteDBSubnetGroupIfManaged(state *StateData) error {
	subnetGroup, err := state.GetParameterValuePtr(ParamDBSubnetGroupName)
	if err != nil {
		return err
	}

	if subnetGroup == nil {
		return nil
	}

	dbSbg, err := p.GetDBSubnetGroup(*subnetGroup)
	if err != nil {
		return err
	}

	p.logger.Logf("Deleting db subnet group: %s", *dbSbg.DBSubnetGroupName)
	p.DeleteDBSubnetGroup(*dbSbg.DBSubnetGroupName)
	return nil
}
