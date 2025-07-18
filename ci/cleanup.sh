#!/usr/bin/env bash

set -uo pipefail            # don’t bail on first error; we’ll retry
trap 'echo "Interrupted – exiting"; exit 1' SIGINT

: "${MAX_RETRIES:=5}"        # overridable:  MAX_RETRIES=10 ./cleanup.sh
: "${RETRY_BASE_DELAY:=2}"   # first sleep in seconds (grows exponentially)

aws() {
  local attempt=1
  while true; do
    command aws "$@" && return 0           # success – done
    if (( attempt >= MAX_RETRIES )); then
      echo "aws $* failed after ${attempt} attempts – skipping" >&2
      return 0                             # carry on; next pass will catch it
    fi
    local delay=$(( RETRY_BASE_DELAY ** attempt ))
    echo "aws $* failed (attempt ${attempt}/${MAX_RETRIES}); retrying in ${delay}s…" >&2
    sleep "${delay}"
    (( attempt++ ))
  done
}


if [[ "$(aws iam list-account-aliases | jq -r '.AccountAliases[0]')" != "convox-ci" ]]; then
  echo "only run this on the ci account"
  exit 1
fi

for region in us-east-1 us-east-2; do
  echo "region: $region"

  echo "deleting cloudformation stacks"
  mapfile -t stacks < <(aws cloudformation list-stacks \
    --region "$region" \
    --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE UPDATE_ROLLBACK_COMPLETE ROLLBACK_COMPLETE IMPORT_COMPLETE UPDATE_ROLLBACK_FAILED \
    --query 'StackSummaries[?starts_with(StackName,`ci-`)].StackName' \
    --output text | tr '\t' '\n')

  if [[ ${#stacks[@]} -eq 0 ]]; then
    echo "   No matching stacks found."
  else
    for stack in "${stacks[@]}"; do
      echo "   Deleting stack $stack…"
      aws cloudformation delete-stack --region "$region" --stack-name "$stack"
    done

    for stack in "${stacks[@]}"; do
      echo "   Waiting for $stack deletion to complete…"
      # The wait command exits with a non‑zero code if the stack disappears
      # before the waiter starts, so we ignore errors.
      aws cloudformation wait stack-delete-complete --region "$region" --stack-name "$stack" || true
    done
  fi

  echo "deleting stucked stacks"
  mapfile -t stacks < <(aws cloudformation list-stacks \
    --region "$region" \
    --stack-status-filter DELETE_FAILED \
    --query 'StackSummaries[?starts_with(StackName,`ci-`)].StackName' \
    --output text | tr '\t' '\n')

  if [[ ${#stacks[@]} -eq 0 ]]; then
    echo "   No matching stacks found."
  else
    for stack in "${stacks[@]}"; do
      echo "   Deleting stack $stack…"
      aws cloudformation delete-stack --region "$region" --stack-name "$stack" --deletion-mode FORCE_DELETE_STACK
    done

    for stack in "${stacks[@]}"; do
      echo "   Waiting for $stack deletion to complete…"
      aws cloudformation wait stack-delete-complete --region "$region" --stack-name "$stack" || true
    done
  fi

  echo "deleting ecr repos"
  for repo in $(aws ecr describe-repositories --region $region | jq -r '.repositories[].repositoryName'); do
    echo "deleting repository: $repo ($region)"
    aws ecr delete-repository --repository-name $repo --region $region --force >/dev/null
  done

  echo "deleting log groups"
  for group in $(aws logs describe-log-groups --region $region | jq -r ".logGroups[].logGroupName"); do
    echo "deleting log group: $group ($region)"
    aws logs delete-log-group --log-group-name $group --region $region
  done

  echo "deleting kms keys"
  for key in $(aws kms list-keys --region $region | jq -r '.Keys[].KeyId'); do
    data=$(aws kms describe-key --region $region --key-id $key)
    state=$(echo $data | jq -r '.KeyMetadata.KeyState')
    manager=$(echo $data | jq -r '.KeyMetadata.KeyManager')
    if [[ "$state" != "PendingDeletion" && "$manager" != "AWS" ]]; then
      echo "deleting key: $key ($region)"
      aws kms schedule-key-deletion --key-id $key --pending-window-in-days 7 --region $region >/dev/null
    fi
  done

  echo "deleting file systems"
  for efs in $(aws efs describe-file-systems --region $region | jq -r '.FileSystems[].FileSystemId'); do
    echo "deleting efs: $efs ($region)"
    for mt in $(aws efs describe-mount-targets --file-system-id $efs --region $region | jq -r '.MountTargets[].MountTargetId'); do
      aws efs delete-mount-target --mount-target-id $mt --region $region
    done
    aws efs delete-file-system --file-system-id $efs --region $region
  done

  echo "deleting eks clusters"
  for eks in $(aws eks list-clusters --region $region | jq -r '.clusters[]'); do
    echo "deleting eks: $eks ($region)"
    for ng in $(aws eks list-nodegroups --cluster-name $eks --region $region | jq -r '.nodegroups[]'); do
      echo "  deleting nodegroup: $ng"
      aws eks delete-nodegroup --cluster-name $eks --nodegroup-name $ng --region $region >/dev/null
    done
    echo "  deleting cluster"
    aws eks delete-cluster --name $eks --region $region >/dev/null
  done

  echo "deleting elbs"
  for elb in $(aws elb describe-load-balancers --region $region | jq -r '.LoadBalancerDescriptions[].LoadBalancerName'); do
    echo "deleting elb: $elb ($region)"
    aws elb delete-load-balancer --load-balancer-name $elb --region $region
  done

  echo "deleting elbv2"
  for alb in $(aws elbv2 describe-load-balancers --region $region | jq -r '.LoadBalancers[].LoadBalancerArn'); do
    echo "deleting alb: $alb ($region)"
    aws elbv2 delete-load-balancer --load-balancer-arn $alb --region $region
  done

  echo "deleting nat gateways"
  for nat in $(aws ec2 describe-nat-gateways --filter Name=state,Values=pending,failed,available --region $region | jq -r '.NatGateways[].NatGatewayId'); do
    echo "deleting nat gateway: $nat ($region)"
    aws ec2 delete-nat-gateway --nat-gateway-id $nat --region $region >/dev/null
  done

  echo "releasing elastic ips"
  for eip in $(aws ec2 describe-addresses --region $region | jq -r '.Addresses[] | select(has("PrivateIpAddress") | not) | .AllocationId'); do
    echo "deleting eip: $eip ($region)"
    aws ec2 release-address --allocation-id $eip --region $region
  done

  echo "deleting internet gateways"
  for igw in $(aws ec2 describe-internet-gateways --region $region | jq -r '.InternetGateways[].InternetGatewayId'); do
      if [[ "$igw" != "igw-0e2ed6542ed5343f2" && "$igw" != "igw-01c3d338eecec02a1" ]]; then # custom ci igws
          echo "deleting igw: $igw ($region)"
          for vpc in $(aws ec2 describe-internet-gateways --internet-gateway-id $igw --region $region | jq -r '.InternetGateways[].Attachments[].VpcId'); do
              aws ec2 detach-internet-gateway --internet-gateway-id $igw --vpc-id $vpc --region $region
          done
          aws ec2 delete-internet-gateway --internet-gateway-id $igw --region $region
      fi
  done

  echo "deleting route tables"
  for rtb in $(aws ec2 describe-route-tables --region $region | jq -r '.RouteTables[] | select(.Associations[0].Main!=true) | .RouteTableId'); do
    echo "deleting route table: $rtb ($region)"
    aws ec2 delete-route-table --route-table-id $rtb --region $region
  done

  echo "deleting security groups and vpcs"
  for vpc in $(aws ec2 describe-vpcs --region $region | jq -r '.Vpcs[].VpcId'); do
      if [[ "$vpc" != "vpc-0f18b6d1265717215" && "$vpc" != "vpc-00e18642ac66249c5" ]]; then # custom ci vpcs
          for sg in $(aws ec2 describe-security-groups --region $region --filters "Name=vpc-id,Values=$vpc" | jq -r '.SecurityGroups[] | select(.GroupName!="default") | .GroupId'); do
            echo "deleting security group: $sg ($region)"
            aws ec2 delete-security-group --group-id $sg --region $region
          done
          echo "deleting vpc: $vpc ($region)"
          aws ec2 delete-vpc --vpc-id $vpc --region $region
      fi
  done

  echo "deleting roles and policies"
  for role in $(aws iam list-roles | jq -r '.Roles[].RoleName'); do
    if [[ "$role" =~ ^ci-[0-9]+ ]]; then
      echo "deleting role: $role"
      for policy in $(aws iam list-attached-role-policies --role-name $role | jq -r '.AttachedPolicies[].PolicyArn'); do
        echo "  detaching policy: $policy"
        aws iam detach-role-policy --role-name $role --policy-arn $policy
      done
      for policy in $(aws iam list-role-policies --role-name $role | jq -r '.PolicyNames[]'); do
        echo "  deleting policy: $policy"
        aws iam delete-role-policy --role-name $role --policy-name $policy
      done
      echo "  deleting role"
      aws iam delete-role --role-name $role
    fi
  done

  echo "deleting open id connect providers"
  for o in $(aws iam list-open-id-connect-providers --no-paginate| jq -r ".OpenIDConnectProviderList[].Arn"); do
    echo "deleting oidc $o"
    aws iam delete-open-id-connect-provider --open-id-connect-provider-arn $o
  done

  echo "deleting enis"
  for eni in $(aws ec2 describe-network-interfaces --region $region | jq -r '.NetworkInterfaces[].NetworkInterfaceId'); do
    echo "deleting network interface: $eni ($region)"

    for eni in $(aws ec2 describe-network-interfaces \
                --region $REGION \
                --query 'NetworkInterfaces[].NetworkInterfaceId' \
                --output text); do

      status=$(aws ec2 describe-network-interfaces \
                  --network-interface-ids "$eni" \
                  --region $REGION \
                  --query 'NetworkInterfaces[0].Status' \
                  --output text)

      if [[ "$status" == "available" ]]; then
          aws ec2 delete-network-interface --network-interface-id "$eni" --region $REGION
          echo "✓ deleted $eni"
      else
          # try a safe detach first
          att_id=$(aws ec2 describe-network-interfaces \
                      --network-interface-ids "$eni" \
                      --region $REGION \
                      --query 'NetworkInterfaces[0].Attachment.AttachmentId' \
                      --output text)

          if [[ "$att_id" != "None" ]]; then
              aws ec2 detach-network-interface --attachment-id "$att_id" --region $REGION --force
              aws ec2 wait network-interface-available --network-interface-ids "$eni" --region $REGION
              aws ec2 delete-network-interface --network-interface-id "$eni" --region $REGION
              echo "✓ detached and deleted $eni"
          else
              echo "⚠ $eni still in use by another AWS service – skipping for now"
          fi
      fi
    done
  done

  echo "deleting subnets"
  for subnet in $(aws ec2 describe-subnets --region $region | jq -r '.Subnets[] | .SubnetId'); do
    echo "deleting subnet: $subnet ($region)"
    aws ec2 delete-subnet --subnet-id $subnet --region $region
  done
done
