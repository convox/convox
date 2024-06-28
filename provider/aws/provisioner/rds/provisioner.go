package rds

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
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
	if v, ok := options[ParamImport]; ok {
		pass := options[ParamMasterUserPassword]
		if pass == "" {
			return fmt.Errorf("imported db password is required for import")
		}
		p.logger.Logf("Start db instance import")
		if err := p.Import(id, v, &pass); err != nil {
			return fmt.Errorf("failed to import db instance: %s", err)
		}
		return nil
	}

	_, err := p.storage.GetState(id)
	if err != nil {
		if IsNotFoundError(err) {
			if options[ParamSourceDBInstanceIdentifier] != "" {
				p.logger.Logf("Start provision for db replica")
				return p.InstallReplica(id, options)
			}

			if options[ParamDBSnapshotIdentifier] != "" {
				p.logger.Logf("Start provision for db instance from snapshot")
				return p.RestoreFromSnapshot(id, options)
			}

			p.logger.Logf("Start provision for db instance")
			if err := p.Install(id, options); err != nil {
				return fmt.Errorf("failed to install db instance: %s", err)
			}
			return nil
		}
		return err
	}

	p.logger.Logf("Start db instance update")
	if err := p.Update(id, options); err != nil {
		return fmt.Errorf("failed to update db instance: %s", err)
	}
	return nil
}

func (p *Provisioner) Install(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !IsNotFoundError(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("already found saved state for this id: %s", id)
	}

	options[ParamDBInstanceIdentifier] = id

	if err := p.ApplyInstallDefaults(options); err != nil {
		return err
	}

	installParamsMeta := GetParametersMetaDataForInstall()

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

	if err := p.createDBSubnetGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create subnet group: %s", err)
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

	if dbCreateResp != nil && dbCreateResp.DBInstance != nil && dbCreateResp.DBInstance.Endpoint != nil &&
		dbCreateResp.DBInstance.Endpoint.Address != nil {
		stateData.Host = *dbCreateResp.DBInstance.Endpoint.Address
	}

	if err := p.SaveState(id, stateData); err != nil {
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	p.logger.Logf("Successfully installed db instance, it may take some times to be available")
	p.storage.SendStateLog(id, "successfully installed db instance")
	return nil
}

func (p *Provisioner) InstallReplica(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !IsNotFoundError(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("already found saved state for this id: %s", id)
	}

	pass := options[ParamMasterUserPassword]
	if pass == "" {
		return fmt.Errorf("source db password is required for read replica")
	}

	options[ParamDBInstanceIdentifier] = id

	installParamsMeta := GetParametersMetaDataForReadReplicaInstall()

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

	p.logger.Logf("Generating the state data for id: %s", id)
	stateData := NewState(id, StateProvisioning, params)

	createOptions, err := stateData.GenerateCreateDBInstanceReadReplicaInput()
	if err != nil {
		return err
	}

	createOptions.Tags = []rdstypes.Tag{
		{
			Key:   aws.String(ProvisionerName),
			Value: aws.String(stateData.Id),
		},
	}

	p.logger.Logf("Installing db instance read replica: %s", id)
	dbCreateResp, err := p.rdsClient.CreateDBInstanceReadReplica(context.TODO(), createOptions)
	if err != nil {
		p.logger.Errorf("Failed to create db instance read replica: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	if dbCreateResp != nil && dbCreateResp.DBInstance != nil && dbCreateResp.DBInstance.Endpoint != nil &&
		dbCreateResp.DBInstance.Endpoint.Address != nil {
		stateData.Host = *dbCreateResp.DBInstance.Endpoint.Address
	}

	if err := p.SaveState(id, stateData); err != nil {
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	p.logger.Logf("Successfully installed db instance read replica, it may take some times to be available")
	p.storage.SendStateLog(id, "successfully installed db instance read replica")
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
		changed, err := stateData.UpdateParameterValueForDbUpdate(k, v)
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

	if targetExistsInStringArray(changedParams, ParamSourceDBInstanceIdentifier) {
		return fmt.Errorf("change in %s parameter not supported", ParamSourceDBInstanceIdentifier)
	}

	modifyReqInput, err := stateData.GenerateModifyDBInstanceInput(changedParams)
	if err != nil {
		return fmt.Errorf("failed to generate modification config: %s", err)
	}

	// TODO: Add option manage this from user side
	modifyReqInput.ApplyImmediately = aws.Bool(true)

	p.logger.Logf("Updating db instance: %s", id)

	_, err = p.rdsClient.ModifyDBInstance(context.Background(), modifyReqInput)
	if err != nil {
		return fmt.Errorf("modification request failed with an error: %s", err)
	}

	if err := p.SaveState(id, stateData); err != nil {
		return err
	}

	p.logger.Logf("Successfully applied the db updates")
	p.storage.SendStateLog(id, "successfully applied the db updates")
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

	if err := p.SaveState(id, state); err != nil {
		return err
	}

	p.logger.Logf("Successfully imported db instance")
	p.storage.SendStateLog(id, "successfully imported db instance")
	return nil
}

func (p *Provisioner) RestoreFromSnapshot(id string, options map[string]string) error {
	_, err := p.storage.GetState(id)
	if err != nil && !IsNotFoundError(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("already found saved state for this id: %s", id)
	}

	options[ParamDBInstanceIdentifier] = id

	if err := p.ApplyRestoreFromSnapshotDefaults(options); err != nil {
		return err
	}

	installParamsMeta := GetParametersMetaDataForRestoreFromSnapshotInstall()

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

	p.logger.Logf("Generating the state data for id: %s", id)
	stateData := NewState(id, StateProvisioning, params)

	if err := p.createSecurityGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create security group: %s", err)
	}

	if err := p.createDBSubnetGroupIfNotProvided(stateData); err != nil {
		return fmt.Errorf("failed to create subnet group: %s", err)
	}

	createOptions, err := stateData.GenerateRestoreDBInstanceFromSnapshotInput()
	if err != nil {
		return err
	}

	createOptions.Tags = []rdstypes.Tag{
		{
			Key:   aws.String(ProvisionerName),
			Value: aws.String(stateData.Id),
		},
	}

	p.logger.Logf("Restoring db instance: %s from snapshot", id)
	dbCreateResp, err := p.rdsClient.RestoreDBInstanceFromDBSnapshot(context.TODO(), createOptions)
	if err != nil {
		p.logger.Errorf("Failed to restore db instance from snapshot: %s", err)
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	if dbCreateResp != nil && dbCreateResp.DBInstance != nil && dbCreateResp.DBInstance.Endpoint != nil &&
		dbCreateResp.DBInstance.Endpoint.Address != nil {
		stateData.Host = *dbCreateResp.DBInstance.Endpoint.Address
	}

	if err := p.SaveState(id, stateData); err != nil {
		p.logger.Logf("Uninstalling because of the error: %s", err)
		p.Uninstall(id)
		return err
	}

	p.logger.Logf("Successfully restored db instance from snapshot, it may take some times to be available")
	p.storage.SendStateLog(id, "successfully restored db instance from snapshot")
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

func (p *Provisioner) SaveState(id string, stateData *StateData) error {
	p.logger.Logf("Saving the state data for id: %s", id)
	stateBytes, err := stateData.GetStateInBytes()
	if err != nil {
		p.logger.Errorf("Failed to get state bytes: %s", err)
		return err
	}

	if err := p.storage.SaveState(id, stateBytes); err != nil {
		p.logger.Errorf("Failed to save state: %s", err)
		return err
	}
	return nil
}

func (p *Provisioner) WaitUntilDBIsAvailable(id string) error {
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

func (p *Provisioner) GetDbStatus(id string) (string, error) {
	stateBytes, err := p.storage.GetState(id)
	if err != nil {
		return "", err
	}

	stateData := NewEmptyState()
	if err := stateData.LoadState(stateBytes); err != nil {
		return "", err
	}

	dbIdentifier, err := stateData.GetParameterValue(ParamDBInstanceIdentifier)
	if err != nil {
		return "", err
	}

	return p.getDbStatus(dbIdentifier)
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

	var (
		user   string
		pass   string
		dbName string
		port   string
	)

	usernamePtr := state.GetParameter(ParamMasterUsername)
	if usernamePtr != nil && !usernamePtr.IsValueEmpty() {
		user = *usernamePtr.Value
	}

	passPtr := state.GetParameter(ParamMasterUserPassword)
	if passPtr != nil && !passPtr.IsValueEmpty() {
		pass = *passPtr.Value
	}

	portPtr := state.GetParameter(ParamPort)
	if portPtr != nil && !portPtr.IsValueEmpty() {
		port = *portPtr.Value
	}

	dbNamePtr := state.GetParameter(ParamDBName)
	if dbNamePtr != nil && !dbNamePtr.IsValueEmpty() {
		dbName = *dbNamePtr.Value
	}

	if state.Host == "" || usernamePtr == nil || portPtr == nil || dbNamePtr == nil {
		if err := p.WaitUntilDBIsAvailable(id); err != nil {
			return nil, err
		}

		dbIdentifier, err := state.GetParameterValue(ParamDBInstanceIdentifier)
		if err != nil {
			return nil, err
		}

		dbResp, err := p.GetDBInstance(dbIdentifier)
		if err != nil {
			return nil, err
		}

		if dbResp.Endpoint == nil || dbResp.Endpoint.Address == nil {
			return nil, fmt.Errorf("db enpoint not found")
		}

		state.Host = *dbResp.Endpoint.Address

		if dbResp.Endpoint == nil || dbResp.Endpoint.Port == nil {
			return nil, fmt.Errorf("db enpoint port not found")
		}
		port = strconv.FormatInt(int64(*dbResp.Endpoint.Port), 10)

		if dbResp.MasterUsername == nil {
			return nil, fmt.Errorf("db master username not found")
		}
		user = *dbResp.MasterUsername

		dbName = GetValueFromStringPtr(dbResp.DBName, "")

		state.AddOrUpdateParameter(*NewParameter(ParamDBName, dbName, &MetaData{}))
		state.AddOrUpdateParameter(*NewParameter(ParamMasterUsername, user, &MetaData{}))
		state.AddOrUpdateParameter(*NewParameter(ParamPort, port, &MetaData{}))

		if err := p.SaveState(id, state); err != nil {
			return nil, err
		}
	}

	return &ConnectionInfo{
		Host:     state.Host,
		Port:     port,
		UserName: user,
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
		if err, ok := err.(awserr.Error); ok && err.Code() == "DBInstanceNotFound" {
			return nil, fmt.Errorf("db instance not found")
		}
		return nil, fmt.Errorf("failed to describe db instances: %s", err)
	}

	if len(result.DBInstances) == 0 {
		return nil, fmt.Errorf("db instance not found")
	}

	return &result.DBInstances[0], nil
}

func (p *Provisioner) DeleteDBInstance(dbIdentifier string) error {
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(dbIdentifier),
	}

	resp, err := p.GetDBInstance(dbIdentifier)
	if err != nil {
		return err
	}

	if resp.ReadReplicaSourceDBInstanceIdentifier != nil && *resp.ReadReplicaSourceDBInstanceIdentifier != "" {
		input.SkipFinalSnapshot = aws.Bool(true)
	} else {
		input.FinalDBSnapshotIdentifier = aws.String(fmt.Sprintf("%s-final-snapshot-%d", dbIdentifier, time.Now().UTC().Unix()))
	}

	_, err = p.rdsClient.DeleteDBInstance(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to delete db instance: %s", err)
	}

	return nil
}

func (p *Provisioner) deleteDBInstancesIfManaged(state *StateData) error {
	dbIdentifier, err := state.GetParameterValue(ParamDBInstanceIdentifier)
	if err != nil {
		return err
	}

	resp, err := p.GetDBInstance(dbIdentifier)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return err
	}

	for _, v := range resp.TagList {
		if v.Key != nil && *v.Key == ProvisionerName && v.Value != nil && *v.Value == state.Id {
			return p.DeleteDBInstance(dbIdentifier)
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
		if IsNotFoundError(err) {
			return nil
		}
		return err
	}

	p.logger.Logf("Deleting db subnet group: %s", *dbSbg.DBSubnetGroupName)
	p.DeleteDBSubnetGroup(*dbSbg.DBSubnetGroupName)
	return nil
}
