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
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	eks "github.com/aws/aws-sdk-go-v2/service/eks"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iam "github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	kms "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/smithy-go"
)

type Options struct {
	Regions    []string
	MaxRetries int
	BaseDelay  time.Duration
}

// allowlistedVPCs are never deleted: they back the existing-vpc install test and must persist.
var allowlistedVPCs = map[string]struct{}{
	"vpc-0f18b6d1265717215": {},
	"vpc-00e18642ac66249c5": {},
}

// DefaultOptions defaults to us-east-1 and us-east-2.
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

// isCITag reports whether the tags positively identify a convox CI resource: the Rack tag
// terraform sets to the rack name (ci-*), or an EKS/CNI cluster tag for a ci-* cluster.
func isCITag(tags []ec2types.Tag) bool {
	for _, t := range tags {
		k, v := aws.ToString(t.Key), aws.ToString(t.Value)
		if k == "Rack" && strings.HasPrefix(v, "ci-") {
			return true
		}
		if strings.HasPrefix(k, "kubernetes.io/cluster/ci-") {
			return true
		}
		if k == "cluster.k8s.amazonaws.com/name" && strings.HasPrefix(v, "ci-") {
			return true
		}
	}
	return false
}

// isCITagELBv2 reports whether load balancer tags identify a convox CI resource: the Rack tag the
// router propagates, or the load balancer controller's cluster tag for a ci-* cluster.
func isCITagELBv2(tags []elbv2types.Tag) bool {
	for _, t := range tags {
		k, v := aws.ToString(t.Key), aws.ToString(t.Value)
		if k == "Rack" && strings.HasPrefix(v, "ci-") {
			return true
		}
		if k == "elbv2.k8s.aws/cluster" && strings.HasPrefix(v, "ci-") {
			return true
		}
		if strings.HasPrefix(k, "kubernetes.io/cluster/ci-") {
			return true
		}
	}
	return false
}

// ciVPCSet returns the ids of CI-owned VPCs in the region: those carrying a ci-* rack tag,
// excluding the account default VPC and the allowlisted VPCs (which always survive).
func ciVPCSet(ctx context.Context, ec2Cl *ec2.Client, o Options) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	pager := ec2.NewDescribeVpcsPaginator(ec2Cl, &ec2.DescribeVpcsInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe vpcs: %w", err)
		}
		for _, vpc := range page.Vpcs {
			vid := aws.ToString(vpc.VpcId)
			if aws.ToBool(vpc.IsDefault) {
				continue
			}
			if _, ok := allowlistedVPCs[vid]; ok {
				continue
			}
			if isCITag(vpc.Tags) {
				set[vid] = struct{}{}
			}
		}
	}
	return set, nil
}

// reapInVPC reports whether a resource in vpcID is in scope: everything inside a CI VPC, and
// only CI-tagged resources inside an allowlisted VPC (so a failed existing-vpc test teardown is
// cleaned without touching the allowlisted VPC itself or any non-CI resource). Resources in the
// default VPC or any other VPC are never in scope.
func reapInVPC(vpcID string, ciVPCs map[string]struct{}, tags []ec2types.Tag) bool {
	if _, ok := ciVPCs[vpcID]; ok {
		return true
	}
	if _, ok := allowlistedVPCs[vpcID]; ok {
		return isCITag(tags)
	}
	return false
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

	log.Printf("Account alias is correct, proceeding with cleanup")

	var cleanupErr error
	for _, region := range opts.Regions {
		region = strings.TrimSpace(region)
		if region == "" {
			continue
		}
		log.Printf("========== REGION: %s ==========", region)

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

		// CI-scoped sweeps identified by name prefix or resource tag (no VPC anchor needed).
		deleteReadyStacks(ctx, cfn, opts)
		forceDeleteFailedStacks(ctx, cfn, opts)
		deleteECR(ctx, ecrCl, opts)
		deleteLogGroups(ctx, logs, opts)
		scheduleKMSDeletion(ctx, kmsCl, opts)
		deleteEFS(ctx, efsCl, opts)
		deleteEKS(ctx, eksCl, opts)
		deleteCIAMRoles(ctx, iamCl, opts)
		deleteOIDCProviders(ctx, iamCl, opts)

		// VPC-anchored teardown: only inside CI VPCs (and CI orphans in the allowlisted VPCs).
		// On a describe failure, skip the network sweep rather than fall back to unfiltered deletes.
		ciVPCs, err := ciVPCSet(ctx, ec2Cl, opts)
		if err != nil {
			log.Printf("   skipping VPC-scoped cleanup in %s: %v", region, err)
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("region %s: %w", region, err))
			continue
		}

		deleteELB(ctx, elbCl, opts, ciVPCs)
		deleteELBV2(ctx, albCl, opts, ciVPCs)
		deleteNATGateways(ctx, ec2Cl, opts, ciVPCs)
		releaseEIPs(ctx, ec2Cl, opts)
		deleteIGWs(ctx, ec2Cl, opts, ciVPCs)
		deleteENIs(ctx, ec2Cl, opts, ciVPCs)
		deleteSubnets(ctx, ec2Cl, opts, ciVPCs)
		// after deleteSubnets: deleting a subnet clears its route-table association
		deleteRouteTables(ctx, ec2Cl, opts, ciVPCs)
		if err := deleteVPCsAndSGs(ctx, ec2Cl, opts, ciVPCs); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("region %s: %w", region, err))
		}
	}

	return cleanupErr
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

		// Bail on non-retriable errors (4xx other than throttling).
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() != "Throttling" {
			return err
		}
		delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if delay <= 0 || delay > 60*time.Second {
			delay = 60 * time.Second
		}
		log.Printf("   retrying after error: %v (attempt %d/%d), sleeping %s", err, attempt, maxRetries, delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("after %d attempts: %w", maxRetries, err)
}

// deleteReadyStacks deletes ci-prefixed stacks in steady states.
func deleteReadyStacks(ctx context.Context, cfn *cf.Client, o Options) {
	log.Println("deleting CloudFormation stacks (steady-state)")

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

// forceDeleteFailedStacks forcibly deletes DELETE_FAILED ci-prefixed stacks.
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
		log.Printf("   force-deleting %s", name)
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

// deleteECR removes ci-prefixed repositories. Repos are <rackname>/<app> or docker-hub-<rackname>/*,
// so a ci- / docker-hub-ci- name prefix identifies CI repos (DescribeRepositories returns no tags).
func deleteECR(ctx context.Context, ecrCl *ecr.Client, o Options) {
	log.Println("deleting ECR repositories")

	reposOut, err := ecrCl.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{})
	if err != nil {
		log.Printf("   describe repos: %v", err)
		return
	}
	for _, repo := range reposOut.Repositories {
		name := aws.ToString(repo.RepositoryName)
		if !strings.HasPrefix(name, "ci-") && !strings.HasPrefix(name, "docker-hub-ci-") {
			continue
		}
		log.Printf("   deleting %s", name)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ecrCl.DeleteRepository(ctx, &ecr.DeleteRepositoryInput{
				RepositoryName: aws.String(name),
				Force:          true,
			})
			return err
		}); err != nil {
			log.Printf("   failed to delete repo %s: %v", name, err)
		}
	}
}

// deleteLogGroups removes CI log groups, identified by a ci- rack/cluster name prefix.
func deleteLogGroups(ctx context.Context, logs *cwlogs.Client, o Options) {
	log.Println("deleting log groups")

	prefixes := []string{"/aws/eks/ci-", "/convox/ci-"}
	for _, prefix := range prefixes {
		pager := cwlogs.NewDescribeLogGroupsPaginator(logs, &cwlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(prefix),
		})
		for pager.HasMorePages() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				log.Printf("   describe log groups (%s): %v", prefix, err)
				break
			}
			for _, lg := range page.LogGroups {
				name := aws.ToString(lg.LogGroupName)
				log.Printf("   deleting %s", name)
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := logs.DeleteLogGroup(ctx, &cwlogs.DeleteLogGroupInput{LogGroupName: aws.String(name)})
					return err
				}); err != nil {
					log.Printf("   failed to delete log group %s: %v", name, err)
				}
			}
		}
	}
}

// scheduleKMSDeletion schedules CI-tagged customer-managed keys for deletion. convox creates no
// customer-managed KMS keys today, so the Rack=ci-* tag gate makes this a safe no-op in practice
// while never touching a non-CI key.
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

		tagsOut, err := kmsCl.ListResourceTags(ctx, &kms.ListResourceTagsInput{KeyId: key.KeyId})
		if err != nil || !kmsTagIsCI(tagsOut.Tags) {
			continue
		}

		log.Printf("   scheduling deletion of key %s", id)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := kmsCl.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
				KeyId:               key.KeyId,
				PendingWindowInDays: aws.Int32(7),
			})
			return err
		}); err != nil {
			log.Printf("   failed to schedule key %s: %v", id, err)
		}
	}
}

func kmsTagIsCI(tags []kmstypes.Tag) bool {
	for _, t := range tags {
		if aws.ToString(t.TagKey) == "Rack" && strings.HasPrefix(aws.ToString(t.TagValue), "ci-") {
			return true
		}
	}
	return false
}

// deleteEFS deletes CI-tagged EFS file systems (and their mount targets).
func deleteEFS(ctx context.Context, efsCl *efs.Client, o Options) {
	log.Println("deleting EFS file systems")

	fsOut, err := efsCl.DescribeFileSystems(ctx, &efs.DescribeFileSystemsInput{})
	if err != nil {
		log.Printf("   describe file systems: %v", err)
		return
	}
	for _, fs := range fsOut.FileSystems {
		if !efsTagIsCI(fs.Tags) {
			continue
		}
		id := aws.ToString(fs.FileSystemId)
		// Delete mount targets first
		mtOut, _ := efsCl.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{FileSystemId: fs.FileSystemId})
		for _, mt := range mtOut.MountTargets {
			mtid := aws.ToString(mt.MountTargetId)
			log.Printf("   deleting mount target %s", mtid)
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := efsCl.DeleteMountTarget(ctx, &efs.DeleteMountTargetInput{MountTargetId: mt.MountTargetId})
				return err
			}); err != nil {
				log.Printf("   failed to delete mount target %s: %v", mtid, err)
			}
		}
		log.Printf("   deleting file system %s", id)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := efsCl.DeleteFileSystem(ctx, &efs.DeleteFileSystemInput{FileSystemId: fs.FileSystemId})
			return err
		}); err != nil {
			log.Printf("   failed to delete file system %s: %v", id, err)
		}
	}
}

func efsTagIsCI(tags []efstypes.Tag) bool {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Rack" && strings.HasPrefix(aws.ToString(t.Value), "ci-") {
			return true
		}
	}
	return false
}

// deleteEKS removes ci-prefixed clusters and their node groups. The cluster name is the rack name.
func deleteEKS(ctx context.Context, eksCl *eks.Client, o Options) {
	log.Println("deleting EKS clusters")

	clustersOut, err := eksCl.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		log.Printf("   list clusters: %v", err)
		return
	}
	for _, name := range clustersOut.Clusters {
		if !strings.HasPrefix(name, "ci-") {
			continue
		}
		log.Printf("   cluster %s", name)

		ngOut, err := eksCl.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(name)})
		if err != nil {
			log.Printf("     failed to list nodegroups: %v", err)
			continue
		}
		if len(ngOut.Nodegroups) > 0 {
			for _, ngName := range ngOut.Nodegroups {
				log.Printf("     deleting nodegroup %s", ngName)
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := eksCl.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{ClusterName: aws.String(name), NodegroupName: aws.String(ngName)})
					return err
				}); err != nil {
					log.Printf("     failed to delete nodegroup %s: %v", ngName, err)
				}
			}
		}
		log.Printf("     deleting cluster %s", name)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := eksCl.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: aws.String(name)})
			return err
		}); err != nil {
			log.Printf("     failed to delete cluster %s: %v", name, err)
		}
	}
}

// deleteELB deletes classic ELBs inside CI VPCs. Convox provisions NLBs (ELBv2) via the load
// balancer controller, so classic ELBs are not expected; allowlisted-VPC orphans are handled in
// deleteELBV2.
func deleteELB(ctx context.Context, elbCl *elb.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting classic ELBs")

	out, err := elbCl.DescribeLoadBalancers(ctx, &elb.DescribeLoadBalancersInput{})
	if err != nil {
		log.Printf("   describe elb: %v", err)
		return
	}
	for _, lb := range out.LoadBalancerDescriptions {
		if _, ok := ciVPCs[aws.ToString(lb.VPCId)]; !ok {
			continue
		}
		name := aws.ToString(lb.LoadBalancerName)
		log.Printf("   deleting %s", name)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := elbCl.DeleteLoadBalancer(ctx, &elb.DeleteLoadBalancerInput{LoadBalancerName: aws.String(name)})
			return err
		}); err != nil {
			log.Printf("   failed to delete elb %s: %v", name, err)
		}
	}
}

// deleteELBV2 deletes ALBs/NLBs inside CI VPCs, plus CI-tagged load balancers orphaned inside an
// allowlisted VPC (a leaked NLB's ENIs keep its CI subnets in use, so it must be reaped first).
func deleteELBV2(ctx context.Context, albCl *elbv2.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting ELBv2 (ALB/NLB)")

	out, err := albCl.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		log.Printf("   describe elbv2: %v", err)
		return
	}
	for _, lb := range out.LoadBalancers {
		arn := aws.ToString(lb.LoadBalancerArn)
		vpcID := aws.ToString(lb.VpcId)
		if _, ok := ciVPCs[vpcID]; !ok {
			if _, ok := allowlistedVPCs[vpcID]; !ok {
				continue
			}
			tagsOut, err := albCl.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{arn}})
			if err != nil || len(tagsOut.TagDescriptions) == 0 || !isCITagELBv2(tagsOut.TagDescriptions[0].Tags) {
				continue
			}
		}
		log.Printf("   deleting %s", arn)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := albCl.DeleteLoadBalancer(ctx, &elbv2.DeleteLoadBalancerInput{LoadBalancerArn: aws.String(arn)})
			return err
		}); err != nil {
			log.Printf("   failed to delete elbv2 %s: %v", arn, err)
		}
	}
}

// deleteNATGateways removes NAT gateways in CI VPCs (and CI orphans in allowlisted VPCs).
func deleteNATGateways(ctx context.Context, ec2Cl *ec2.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting NAT gateways")

	pager := ec2.NewDescribeNatGatewaysPaginator(ec2Cl, &ec2.DescribeNatGatewaysInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe nat: %v", err)
			return
		}
		for _, ng := range page.NatGateways {
			if !reapInVPC(aws.ToString(ng.VpcId), ciVPCs, ng.Tags) {
				continue
			}
			id := aws.ToString(ng.NatGatewayId)
			log.Printf("   deleting %s", id)
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(id)})
				return err
			}); err != nil {
				log.Printf("   failed to delete nat %s: %v", id, err)
			}
		}
	}
}

// releaseEIPs frees unattached CI-tagged Elastic IPs (the EIPs terraform allocates for NAT gateways).
// A NAT gateway deleted earlier in the same run still shows its EIP associated, so that EIP is
// released on the next daily run once the association clears.
func releaseEIPs(ctx context.Context, ec2Cl *ec2.Client, o Options) {
	log.Println("releasing Elastic IPs")

	out, err := ec2Cl.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		log.Printf("   describe eips: %v", err)
		return
	}
	for _, addr := range out.Addresses {
		if addr.AssociationId != nil {
			continue // skip those still attached
		}
		if !isCITag(addr.Tags) {
			continue
		}
		alloc := aws.ToString(addr.AllocationId)
		log.Printf("   releasing %s", alloc)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(alloc)})
			return err
		}); err != nil {
			log.Printf("   failed to release eip %s: %v", alloc, err)
		}
	}
}

// deleteIGWs detaches and deletes internet gateways attached to CI VPCs.
func deleteIGWs(ctx context.Context, ec2Cl *ec2.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting Internet gateways")

	pager := ec2.NewDescribeInternetGatewaysPaginator(ec2Cl, &ec2.DescribeInternetGatewaysInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe igw: %v", err)
			return
		}
		for _, igw := range page.InternetGateways {
			reap := false
			for _, att := range igw.Attachments {
				if reapInVPC(aws.ToString(att.VpcId), ciVPCs, igw.Tags) {
					reap = true
					break
				}
			}
			if !reap {
				continue
			}
			id := aws.ToString(igw.InternetGatewayId)
			log.Printf("   deleting %s", id)
			// detach from VPCs first
			for _, att := range igw.Attachments {
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := ec2Cl.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
						InternetGatewayId: igw.InternetGatewayId,
						VpcId:             att.VpcId,
					})
					return err
				}); err != nil {
					log.Printf("   failed to detach igw %s: %v", id, err)
				}
			}
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{InternetGatewayId: igw.InternetGatewayId})
				return err
			}); err != nil {
				log.Printf("   failed to delete igw %s: %v", id, err)
			}
		}
	}
}

// deleteRouteTables removes non-main route tables in CI VPCs (and CI orphans in allowlisted VPCs).
func deleteRouteTables(ctx context.Context, ec2Cl *ec2.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting route tables")

	pager := ec2.NewDescribeRouteTablesPaginator(ec2Cl, &ec2.DescribeRouteTablesInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe rtb: %v", err)
			return
		}
		for _, rtb := range page.RouteTables {
			if !reapInVPC(aws.ToString(rtb.VpcId), ciVPCs, rtb.Tags) {
				continue
			}
			if len(rtb.Associations) > 0 && rtb.Associations[0].Main != nil && *rtb.Associations[0].Main {
				continue // skip main tables
			}
			id := aws.ToString(rtb.RouteTableId)
			log.Printf("   deleting %s", id)
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{RouteTableId: rtb.RouteTableId})
				return err
			}); err != nil {
				log.Printf("   failed to delete rtb %s: %v", id, err)
			}
		}
	}
}

// deleteVPCsAndSGs deletes each CI VPC (non-default SGs then the VPC), and reaps CI-tagged security
// groups orphaned inside the allowlisted VPCs without deleting those VPCs. Returns the joined set of
// VPC-deletion failures so the caller can surface a non-zero exit.
func deleteVPCsAndSGs(ctx context.Context, ec2Cl *ec2.Client, o Options, ciVPCs map[string]struct{}) error {
	log.Println("deleting VPCs and security groups")

	// CI orphans inside the allowlisted VPCs (never the VPCs themselves).
	for allowVPC := range allowlistedVPCs {
		deleteSGsInVPC(ctx, ec2Cl, o, allowVPC, true)
	}

	var errs []error
	for vid := range ciVPCs {
		deleteSGsInVPC(ctx, ec2Cl, o, vid, false)
		log.Printf("   deleting VPC %s", vid)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := ec2Cl.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String(vid)})
			return err
		}); err != nil {
			log.Printf("   failed to delete VPC %s: %v", vid, err)
			errs = append(errs, fmt.Errorf("vpc %s: %w", vid, err))
		}
	}
	return errors.Join(errs...)
}

// deleteSGsInVPC deletes non-default security groups in vpcID. When onlyCITagged is set, only
// CI-tagged groups are removed (used inside allowlisted VPCs to spare non-CI groups).
func deleteSGsInVPC(ctx context.Context, ec2Cl *ec2.Client, o Options, vpcID string, onlyCITagged bool) {
	pager := ec2.NewDescribeSecurityGroupsPaginator(ec2Cl, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcID},
		}},
	})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe sgs %s: %v", vpcID, err)
			return
		}
		for _, sg := range page.SecurityGroups {
			if aws.ToString(sg.GroupName) == "default" {
				continue
			}
			if onlyCITagged && !isCITag(sg.Tags) {
				continue
			}
			sgid := aws.ToString(sg.GroupId)
			log.Printf("   deleting SG %s", sgid)
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: sg.GroupId})
				return err
			}); err != nil {
				log.Printf("   failed to delete SG %s: %v", sgid, err)
			}
		}
	}
}

// deleteCIAMRoles deletes iam roles that match ci- and their inline/attached policies.
func deleteCIAMRoles(ctx context.Context, iamCl *iam.Client, o Options) {
	log.Println("deleting IAM roles + policies")

	pager := iam.NewListRolesPaginator(iamCl, &iam.ListRolesInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   list roles: %v", err)
			return
		}
		for _, role := range page.Roles {
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
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := iamCl.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: aws.String(name), PolicyArn: p.PolicyArn})
					return err
				}); err != nil {
					log.Printf("     failed to detach %s: %v", arn, err)
				}
			}
			// 2. delete inline policies
			ips, _ := iamCl.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: aws.String(name)})
			for _, pname := range ips.PolicyNames {
				log.Printf("     deleting inline policy %s", pname)
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := iamCl.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{RoleName: aws.String(name), PolicyName: aws.String(pname)})
					return err
				}); err != nil {
					log.Printf("     failed to delete inline policy %s: %v", pname, err)
				}
			}
			// 3. finally delete role
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := iamCl.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: aws.String(name)})
				return err
			}); err != nil {
				log.Printf("   failed to delete role %s: %v", name, err)
			}
		}
	}
}

// deleteOIDCProviders removes CI-tagged OIDC providers (terraform tags the cluster provider Rack=ci-*).
func deleteOIDCProviders(ctx context.Context, iamCl *iam.Client, o Options) {
	log.Println("deleting OIDC providers")

	out, err := iamCl.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		log.Printf("   list oidc providers: %v", err)
		return
	}
	for _, p := range out.OpenIDConnectProviderList {
		arn := aws.ToString(p.Arn)
		desc, err := iamCl.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{OpenIDConnectProviderArn: p.Arn})
		if err != nil {
			log.Printf("   get oidc %s: %v", arn, err)
			continue
		}
		if !iamTagIsCI(desc.Tags) {
			continue
		}
		log.Printf("   deleting %s", arn)
		if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
			_, err := iamCl.DeleteOpenIDConnectProvider(ctx, &iam.DeleteOpenIDConnectProviderInput{OpenIDConnectProviderArn: p.Arn})
			return err
		}); err != nil {
			log.Printf("   failed to delete oidc %s: %v", arn, err)
		}
	}
}

func iamTagIsCI(tags []iamtypes.Tag) bool {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Rack" && strings.HasPrefix(aws.ToString(t.Value), "ci-") {
			return true
		}
	}
	return false
}

// deleteENIs deletes or detaches/then deletes network interfaces in CI VPCs.
func deleteENIs(ctx context.Context, ec2Cl *ec2.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting ENIs")

	waiter := ec2.NewNetworkInterfaceAvailableWaiter(ec2Cl)
	pager := ec2.NewDescribeNetworkInterfacesPaginator(ec2Cl, &ec2.DescribeNetworkInterfacesInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe enis: %v", err)
			return
		}
		for _, eni := range page.NetworkInterfaces {
			if !reapInVPC(aws.ToString(eni.VpcId), ciVPCs, eni.TagSet) {
				continue
			}
			id := aws.ToString(eni.NetworkInterfaceId)
			if eni.Status == ec2types.NetworkInterfaceStatusAvailable {
				log.Printf("   deleting %s", id)
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := ec2Cl.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: eni.NetworkInterfaceId})
					return err
				}); err != nil {
					log.Printf("   failed to delete eni %s: %v", id, err)
				}
				continue
			}
			// otherwise detach first (if possible)
			if eni.Attachment != nil && eni.Attachment.AttachmentId != nil {
				att := aws.ToString(eni.Attachment.AttachmentId)
				log.Printf("   detaching %s (attachment %s)", id, att)
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := ec2Cl.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{AttachmentId: aws.String(att), Force: aws.Bool(true)})
					return err
				}); err != nil {
					log.Printf("   failed to detach eni %s: %v", id, err)
				}
				_ = waiter.Wait(ctx, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: []string{id}}, 5*time.Minute)
				if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
					_, err := ec2Cl.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: aws.String(id)})
					return err
				}); err != nil {
					log.Printf("   failed to delete eni %s: %v", id, err)
				}
				continue
			}
			log.Printf("   %s still in use, skipping", id)
		}
	}
}

// deleteSubnets removes subnets in CI VPCs (and CI orphans in allowlisted VPCs).
func deleteSubnets(ctx context.Context, ec2Cl *ec2.Client, o Options, ciVPCs map[string]struct{}) {
	log.Println("deleting subnets")

	pager := ec2.NewDescribeSubnetsPaginator(ec2Cl, &ec2.DescribeSubnetsInput{})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("   describe subnets: %v", err)
			return
		}
		for _, sn := range page.Subnets {
			if !reapInVPC(aws.ToString(sn.VpcId), ciVPCs, sn.Tags) {
				continue
			}
			id := aws.ToString(sn.SubnetId)
			log.Printf("   deleting %s", id)
			if err := withRetry(ctx, o.MaxRetries, o.BaseDelay, func() error {
				_, err := ec2Cl.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: sn.SubnetId})
				return err
			}); err != nil {
				log.Printf("   failed to delete subnet %s: %v", id, err)
			}
		}
	}
}
