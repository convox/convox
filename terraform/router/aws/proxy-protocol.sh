#!/bin/sh

set -e

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

target_groups=$(aws --region=$3 resourcegroupstaggingapi get-resources --tag-filters "Key=kubernetes.io/service-name,Values=$1-system/router" --resource-type-filters "elasticloadbalancing:targetgroup")
target_groups=$(echo $target_groups | jq -r "[.ResourceTagMappingList[] | .ResourceARN] | @sh" | tr -d \')

seconds=0
while [ -z "$target_groups" ]
do
    seconds=$(($seconds+30))
    if [ $seconds -ge 600 ]
    then
        echo "failed to provision load-balancer for $1"
        exit 1
    fi
    echo "waiting for NLB target-groups..."
    sleep 30
    target_groups=$(aws resourcegroupstaggingapi get-resources --tag-filters "Key=kubernetes.io/service-name,Values=$1-system/router" --resource-type-filters "elasticloadbalancing:targetgroup")
    target_groups=$(echo $target_groups | jq -r "[.ResourceTagMappingList[] | .ResourceARN] | @sh" | tr -d \')
done

for group in $target_groups; do
    aws --region=$3 elbv2 modify-target-group-attributes --target-group-arn $group --attributes Key=proxy_protocol_v2.enabled,Value=$2 > /dev/null
done
