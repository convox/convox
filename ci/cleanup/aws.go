package cleanup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecr "github.com/aws/aws-sdk-go-v2/service/ecr"
	efs "github.com/aws/aws-sdk-go-v2/service/efs"
	eks "github.com/aws/aws-sdk-go-v2/service/eks"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	iam "github.com/aws/aws-sdk-go-v2/service/iam"
	kms "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/smithy-go"
)

type Options struct {
	Regions    []string
	MaxRetries int
	BaseDelay  time.Duration
}

// DefaultOptions defaults to us‑east‑1 and us‑east‑2.
func DefaultOptions() Options {
	// Regions
	reg := []string{"us-east-1", "us-east-2"}
	if v := os.Getenv("REGIONS"); v != "" {
		reg = strings.Split(v, ",")
	}

	// Retries
	retries := 5
	if v := os.Getenv("MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retries = n
		}
	}

	// Base delay
	delay := 2 * time.Second
	if v := os.Getenv("RETRY_BASE_DELAY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			delay = time.Duration(n) * time.Second
		}
	}

	return Options{
		Regions:    reg,
		MaxRetries: retries,
		BaseDelay:  delay,
	}
}

func Run(ctx context.Context, opts Options) error {
	if len(opts.Regions) == 0 {
		opts.Regions = []string{"us-east-1", "us-east-2"}
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 5
	}
	if opts.BaseDelay <= 0 {
		opts.BaseDelay = 2 * time.Second
	}

	rootCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	if err := assertCIAlias(ctx, iam.NewFromConfig(rootCfg)); err != nil {
		return err
	}

	log.Printf("Account alias is correct – proceeding with cleanup")

	for _, region := range opts.Regions {
		region = strings.TrimSpace(region)
		if region == "" {
			continue
		}
		log.Printf("────────── REGION: %s ──────────", region)

		cfg := rootCfg.Copy()
		cfg.Region = region

		cfn := cf.NewFromConfig(cfg)
		ecrCl := ecr.NewFromConfig(cfg)
		logs := cwlogs.NewFromConfig(cfg)
		kmsCl := kms.NewFromConfig(cfg)
		efsCl := efs.NewFromConfig(cfg)
		eksCl := eks.NewFromConfig(cfg)
		elbCl := elb.NewFromConfig(cfg)
		albCl := elbv2.NewFromConfig(cfg)
		ec2Cl := ec2.NewFromConfig(cfg)
		iamCl := iam.NewFromConfig(cfg)

		deleteReadyStacks(ctx, cfn, opts)
		forceDeleteFailedStacks(ctx, cfn, opts)

		deleteECR(ctx, ecrCl, opts)

		deleteLogGroups(ctx, logs, opts)

		scheduleKMSDeletion(ctx, kmsCl, opts)

		deleteEFS(ctx, efsCl, opts)

		deleteEKS(ctx, eksCl, opts)

		deleteELB(ctx, elbCl, opts)

		deleteELBV2(ctx, albCl, opts)

		deleteNATGateways(ctx, ec2Cl, opts)

		releaseEIPs(ctx, ec2Cl, opts)

		deleteIGWs(ctx, ec2Cl, opts)

		deleteRouteTables(ctx, ec2Cl, opts)

		deleteVPCsAndSGs(ctx, ec2Cl, opts)

		deleteCIAMRoles(ctx, iamCl, opts)

		deleteOIDCProviders(ctx, iamCl, opts)

		deleteENIs(ctx, ec2Cl, opts)

		deleteSubnets(ctx, ec2Cl, opts)
	}

	return nil
}

func assertCIAlias(ctx context.Context, iamCl *iam.Client) error {
	out, err := iamCl.ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})
	if err != nil {
		return fmt.Errorf("list account aliases: %w", err)
	}
	if len(out.AccountAliases) == 0 || out.AccountAliases[0] != "convox-ci" {
		return errors.New("only run this on the ci account")
	}
	return nil
}

// withRetry executes a function with retries, handling throttling and other retriable errors.
func withRetry(ctx context.Context, maxRetries int, baseDelay time.Duration, f func() error) error {
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = f()
		if err == nil {
			return nil
		}

		// If the error is non‑retriable (4xx other than throttling), bail.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() != "Throttling" {
			return err
		}
		delay := time.Duration(math.Pow(float64(baseDelay), float64(attempt)))
		log.Printf("   retrying after error: %v (attempt %d/%d) – sleeping %s", err, attempt, maxRetries, delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("after %d attempts: %w", maxRetries, err)
}

// deleteReadyStacks deletes CI‑prefixed stacks in steady states.
func deleteReadyStacks(ctx context.Context, cfn *cf.Client, o Options) {
	log.Println("deleting CloudFormation stacks (steady‑state)")

	stacksOut, err := cfn.ListStacks(ctx, &cf.ListStacksInput{
		StackStatusFilter: []cftypes.StackStatus{
			cftypes.StackStatusCreateComplete,
			cftypes.StackStatusUpdateComplete,
			cftypes.StackStatusUpdateRollbackComplete,
			cftypes.StackStatusRollbackComplete,
			cftypes.StackStatusImportComplete,
			cftypes.StackStatusUpdateRollbackFailed,
			cftypes.StackStatusCreateFailed,
			cftypes.StackStatusRollbackFailed,
			cftypes.StackStatusUpdateFailed,
			cftypes.StackStatusUpdateRollbackFailed,
			cftypes.StackStatusImportRollbackFailed,
		},
	})
	if err != nil {
		log.Printf("   list stacks: %v", err)
		return
	}

	var targets []string
	for _, s := range stacksOut.StackSummaries {
		name := aws.ToString(s.StackName)
		if strings.HasPrefix(name, "ci-") {
			targets = append(targets, name)
		}
	}
	if len(targets) == 0 {
		log.Println("   no matching stacks found")
		return
	}

	for _, name := range targets {
		log.Printf("   deleting %s", name)
		err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := cfn.DeleteStack(ctx, &cf.DeleteStackInput{StackName: aws.String(name)})
			return err
		})
		if err != nil {
			log.Printf("   failed to delete %s: %v", name, err)
			continue
		}
	}

	waiter := cf.NewStackDeleteCompleteWaiter(cfn)
	for _, name := range targets {
		log.Printf("   waiting for %s deletion", name)
		err := waiter.Wait(ctx, &cf.DescribeStacksInput{StackName: aws.String(name)}, 15*time.Minute)
		if err != nil {
			log.Printf("   failed to wait for %s deletion: %v", name, err)
			continue
		}
	}
}

// forceDeleteFailedStacks forcibly deletes DELETE_FAILED stacks.
func forceDeleteFailedStacks(ctx context.Context, cfn *cf.Client, o Options) {
	log.Println("deleting CloudFormation stacks (DELETE_FAILED)")

	stacksOut, err := cfn.ListStacks(ctx, &cf.ListStacksInput{
		StackStatusFilter: []cftypes.StackStatus{
			cftypes.StackStatusDeleteFailed,
		},
	})
	if err != nil {
		log.Printf("   list stacks: %v", err)
		return
	}

	var targets []string
	for _, s := range stacksOut.StackSummaries {
		name := aws.ToString(s.StackName)
		if strings.HasPrefix(name, "ci-") {
			targets = append(targets, name)
		}
	}
	if len(targets) == 0 {
		log.Println("   no matching stacks found")
		return
	}

	for _, name := range targets {
		log.Printf("   force‑deleting %s", name)
		err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := cfn.DeleteStack(ctx, &cf.DeleteStackInput{
				StackName:    aws.String(name),
				DeletionMode: cftypes.DeletionModeForceDeleteStack,
			})
			return err
		})
		if err != nil {
			log.Printf("   failed to delete %s: %v", name, err)
			continue
		}
	}
}

// deleteECR empties and removes all repositories in a region.
func deleteECR(ctx context.Context, ecrCl *ecr.Client, o Options) {
	log.Println("deleting ECR repositories")

	reposOut, err := ecrCl.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{})
	if err != nil {
		log.Printf("   describe repos: %v", err)
		return
	}
	for _, repo := range reposOut.Repositories {
		name := aws.ToString(repo.RepositoryName)
		log.Printf("   deleting %s", name)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ecrCl.DeleteRepository(ctx, &ecr.DeleteRepositoryInput{
				RepositoryName: aws.String(name),
				Force:          true,
			})
			return err
		})
	}
}

// deleteLogGroups removes every CloudWatch log group.
func deleteLogGroups(ctx context.Context, logs *cwlogs.Client, o Options) {
	log.Println("deleting log groups")

	pager := cwlogs.NewDescribeLogGroupsPaginator(logs, &cwlogs.DescribeLogGroupsInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe log groups: %v", err)
			return
		}
		for _, lg := range page.LogGroups {
			name := aws.ToString(lg.LogGroupName)
			log.Printf("   deleting %s", name)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := logs.DeleteLogGroup(ctx, &cwlogs.DeleteLogGroupInput{LogGroupName: aws.String(name)})
				return err
			})
		}
	}
}

// scheduleKMSDeletion schedules all customer-managed keys for deletion.
func scheduleKMSDeletion(ctx context.Context, kmsCl *kms.Client, o Options) {
	log.Println("scheduling KMS keys for deletion")

	keysOut, err := kmsCl.ListKeys(ctx, &kms.ListKeysInput{})
	if err != nil {
		log.Printf("   list keys: %v", err)
		return
	}
	for _, key := range keysOut.Keys {
		id := aws.ToString(key.KeyId)

		desc, err := kmsCl.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: key.KeyId})
		if err != nil {
			continue
		}
		md := desc.KeyMetadata
		if md == nil ||
			md.KeyManager == kmstypes.KeyManagerTypeAws ||
			md.KeyState == kmstypes.KeyStatePendingDeletion {
			continue
		}

		log.Printf("   scheduling deletion of key %s", id)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := kmsCl.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
				KeyId:               key.KeyId,
				PendingWindowInDays: aws.Int32(7),
			})
			return err
		})
	}
}

// deleteEFS deletes all EFS file systems (and mount targets).
func deleteEFS(ctx context.Context, efsCl *efs.Client, o Options) {
	log.Println("deleting EFS file systems")

	fsOut, err := efsCl.DescribeFileSystems(ctx, &efs.DescribeFileSystemsInput{})
	if err != nil {
		log.Printf("   describe file systems: %v", err)
		return
	}
	for _, fs := range fsOut.FileSystems {
		id := aws.ToString(fs.FileSystemId)
		// Delete mount targets first
		mtOut, _ := efsCl.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{FileSystemId: fs.FileSystemId})
		for _, mt := range mtOut.MountTargets {
			mtid := aws.ToString(mt.MountTargetId)
			log.Printf("   deleting mount target %s", mtid)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := efsCl.DeleteMountTarget(ctx, &efs.DeleteMountTargetInput{MountTargetId: mt.MountTargetId})
				return err
			})
		}
		log.Printf("   deleting file system %s", id)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := efsCl.DeleteFileSystem(ctx, &efs.DeleteFileSystemInput{FileSystemId: fs.FileSystemId})
			return err
		})
	}
}

// deleteEKS removes clusters and their node groups.
func deleteEKS(ctx context.Context, eksCl *eks.Client, o Options) {
	log.Println("deleting EKS clusters")

	clustersOut, err := eksCl.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		log.Printf("   list clusters: %v", err)
		return
	}
	for _, name := range clustersOut.Clusters {
		log.Printf("   cluster %s", name)

		ngOut, err := eksCl.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(name)})
		if err != nil {
			log.Printf("     failed to list nodegroups: %v", err)
			continue
		}
		if len(ngOut.Nodegroups) > 0 {
			for _, ngName := range ngOut.Nodegroups {
				log.Printf("     deleting nodegroup %s", ngName)
				withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := eksCl.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{ClusterName: aws.String(name), NodegroupName: aws.String(ngName)})
					return err
				})
			}
		}
		log.Printf("     deleting cluster %s", name)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := eksCl.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: aws.String(name)})
			return err
		})
	}
}

// deleteELB deletes classic ELBs.
func deleteELB(ctx context.Context, elbCl *elb.Client, o Options) {
	log.Println("deleting classic ELBs")

	out, err := elbCl.DescribeLoadBalancers(ctx, &elb.DescribeLoadBalancersInput{})
	if err != nil {
		log.Printf("   describe elb: %v", err)
		return
	}
	for _, lb := range out.LoadBalancerDescriptions {
		name := aws.ToString(lb.LoadBalancerName)
		log.Printf("   deleting %s", name)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := elbCl.DeleteLoadBalancer(ctx, &elb.DeleteLoadBalancerInput{LoadBalancerName: aws.String(name)})
			return err
		})
	}
}

// deleteELBV2 deletes ALBs/NLBs.
func deleteELBV2(ctx context.Context, albCl *elbv2.Client, o Options) {
	log.Println("deleting ELBv2 (ALB/NLB)")

	out, err := albCl.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		log.Printf("   describe elbv2: %v", err)
		return
	}
	for _, lb := range out.LoadBalancers {
		arn := aws.ToString(lb.LoadBalancerArn)
		log.Printf("   deleting %s", arn)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := albCl.DeleteLoadBalancer(ctx, &elbv2.DeleteLoadBalancerInput{LoadBalancerArn: aws.String(arn)})
			return err
		})
	}
}

// deleteNATGateways removes pending/failed/available NAT gateways.
func deleteNATGateways(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("deleting NAT gateways")

	out, err := ec2Cl.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{})
	if err != nil {
		log.Printf("   describe nat: %v", err)
		return
	}
	for _, ng := range out.NatGateways {
		id := aws.ToString(ng.NatGatewayId)
		log.Printf("   deleting %s", id)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(id)})
			return err
		})
	}
}

// releaseEIPs frees Elastic IP addresses without a private IP.
func releaseEIPs(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("releasing Elastic IPs")

	out, err := ec2Cl.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		log.Printf("   describe eips: %v", err)
		return
	}
	for _, addr := range out.Addresses {
		if addr.PrivateIpAddress != nil {
			continue // skip those still attached
		}
		alloc := aws.ToString(addr.AllocationId)
		log.Printf("   releasing %s", alloc)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(alloc)})
			return err
		})
	}
}

// deleteIGWs deletes all IGWs except the hard‑coded CI ones.
func deleteIGWs(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("deleting Internet gateways")

	out, err := ec2Cl.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{})
	if err != nil {
		log.Printf("   describe igw: %v", err)
		return
	}
	skip := map[string]struct{}{
		"igw-0e2ed6542ed5343f2": {},
		"igw-01c3d338eecec02a1": {},
	}
	for _, igw := range out.InternetGateways {
		id := aws.ToString(igw.InternetGatewayId)
		if _, ok := skip[id]; ok {
			continue
		}
		log.Printf("   deleting %s", id)
		// detach from VPCs first
		for _, att := range igw.Attachments {
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
					VpcId:             att.VpcId,
				})
				return err
			})
		}
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{InternetGatewayId: igw.InternetGatewayId})
			return err
		})
	}
}

// deleteRouteTables removes all non‑main route tables.
func deleteRouteTables(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("deleting route tables")

	out, err := ec2Cl.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{})
	if err != nil {
		log.Printf("   describe rtb: %v", err)
		return
	}
	for _, rtb := range out.RouteTables {
		id := aws.ToString(rtb.RouteTableId)
		if len(rtb.Associations) > 0 && rtb.Associations[0].Main != nil && *rtb.Associations[0].Main {
			continue // skip main tables
		}
		log.Printf("   deleting %s", id)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{RouteTableId: rtb.RouteTableId})
			return err
		})
	}
}

// deleteVPCsAndSGs removes security groups (non‑default) then VPCs.
func deleteVPCsAndSGs(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("deleting VPCs and security groups")

	skip := map[string]struct{}{
		"vpc-0f18b6d1265717215": {},
		"vpc-00e18642ac66249c5": {},
	}

	vpcsOut, err := ec2Cl.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		log.Printf("   describe vpcs: %v", err)
		return
	}
	for _, vpc := range vpcsOut.Vpcs {
		vid := aws.ToString(vpc.VpcId)
		if _, ok := skip[vid]; ok {
			continue
		}
		// Delete SGs first (non‑default)
		sgsOut, _ := ec2Cl.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{{
				Name:   aws.String("vpc-id"),
				Values: []string{vid},
			}},
		})
		for _, sg := range sgsOut.SecurityGroups {
			if aws.ToString(sg.GroupName) == "default" {
				continue
			}
			sgid := aws.ToString(sg.GroupId)
			log.Printf("   deleting SG %s", sgid)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: sg.GroupId})
				return err
			})
		}
		// Now delete VPC
		log.Printf("   deleting VPC %s", vid)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: vpc.VpcId})
			return err
		})
	}
}

// deleteCIAMRoles deletes iam roles that match ^ci-[0-9]+ and their inline/attached policies.
func deleteCIAMRoles(ctx context.Context, iamCl *iam.Client, o Options) {
	log.Println("deleting IAM roles + policies")

	rolesOut, err := iamCl.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		log.Printf("   list roles: %v", err)
		return
	}
	for _, role := range rolesOut.Roles {
		name := aws.ToString(role.RoleName)
		if !strings.HasPrefix(name, "ci-") {
			continue
		}
		log.Printf("   deleting role %s", name)
		// 1. detach attached policies
		atts, _ := iamCl.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{RoleName: aws.String(name)})
		for _, p := range atts.AttachedPolicies {
			arn := aws.ToString(p.PolicyArn)
			log.Printf("     detaching %s", arn)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := iamCl.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: aws.String(name), PolicyArn: p.PolicyArn})
				return err
			})
		}
		// 2. delete inline policies
		ips, _ := iamCl.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: aws.String(name)})
		for _, pname := range ips.PolicyNames {
			log.Printf("     deleting inline policy %s", pname)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := iamCl.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{RoleName: aws.String(name), PolicyName: aws.String(pname)})
				return err
			})
		}
		// 3. finally delete role
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := iamCl.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: aws.String(name)})
			return err
		})
	}
}

// deleteOIDCProviders removes every OIDC provider.
func deleteOIDCProviders(ctx context.Context, iamCl *iam.Client, o Options) {
	log.Println("deleting OIDC providers")

	out, err := iamCl.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		log.Printf("   list oidc providers: %v", err)
		return
	}
	for _, p := range out.OpenIDConnectProviderList {
		arn := aws.ToString(p.Arn)
		log.Printf("   deleting %s", arn)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := iamCl.DeleteOpenIDConnectProvider(ctx, &iam.DeleteOpenIDConnectProviderInput{OpenIDConnectProviderArn: p.Arn})
			return err
		})
	}
}

// deleteENIs deletes or detaches/then deletes network interfaces.
func deleteENIs(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("deleting ENIs")

	out, err := ec2Cl.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{})
	if err != nil {
		log.Printf("   describe enis: %v", err)
		return
	}
	waiter := ec2.NewNetworkInterfaceAvailableWaiter(ec2Cl)
	for _, eni := range out.NetworkInterfaces {
		id := aws.ToString(eni.NetworkInterfaceId)
		if eni.Status == ec2types.NetworkInterfaceStatusAvailable {
			log.Printf("   deleting %s", id)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: eni.NetworkInterfaceId})
				return err
			})
			continue
		}
		// otherwise detach first (if possible)
		if eni.Attachment != nil && eni.Attachment.AttachmentId != nil {
			att := aws.ToString(eni.Attachment.AttachmentId)
			log.Printf("   detaching %s (attachment %s)", id, att)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{AttachmentId: aws.String(att), Force: aws.Bool(true)})
				return err
			})
			_ = waiter.Wait(ctx, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: []string{id}}, 5*time.Minute)
			withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: aws.String(id)})
				return err
			})
			continue
		}
		log.Printf("   ⚠ %s still in use – skipping", id)
	}
}

// deleteSubnets removes every subnet.
func deleteSubnets(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("deleting subnets")

	out, err := ec2Cl.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		log.Printf("   describe subnets: %v", err)
		return
	}
	for _, sn := range out.Subnets {
		id := aws.ToString(sn.SubnetId)
		log.Printf("   deleting %s", id)
		withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: sn.SubnetId})
			return err
		})
	}
}
