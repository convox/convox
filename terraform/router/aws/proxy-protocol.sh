#!/bin/sh

set -e

export AWS_REGION=us-east-1

echo "the new env $AWS_REGION"

if [ ! -x "$(which jq)" ]
then
    echo "missing jq binary"
    exit 1
fi

if [ ! -x "$(which aws)" ]
then
    echo "missing aws cli"
    exit 1
fi

if [ ! -x "$(which tr)" ]
then
    echo "missing tr binary"
    exit 1
fi

sleep 5 # make sure the load-balancer and target groups associated with the router service were already created 

target_groups=$(aws resourcegroupstaggingapi get-resources --tag-filters "Key=kubernetes.io/service-name,Values=$1-system/router" --resource-type-filters "elasticloadbalancing:targetgroup")

target_groups=$(echo $target_groups | jq -r "[.ResourceTagMappingList[] | .ResourceARN] | @sh" | tr -d \')

if [ -z "$target_groups" ]
then
  echo "router for rack $1 not found"
  exit 1
fi

for group in $target_groups; do
    aws elbv2 modify-target-group-attributes --target-group-arn $group --attributes Key=proxy_protocol_v2.enabled,Value=$2 > /dev/null
done
