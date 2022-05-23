#!/bin/sh

set -e

sleep 5 # make sure the load-balancer and target groups associated with the router service were already created 

tgroups=$(aws resourcegroupstaggingapi get-resources --tag-filters "Key=kubernetes.io/service-name,Values=$1-system/router" --resource-type-filters "elasticloadbalancing:targetgroup" | jq -r "[.ResourceTagMappingList[] | .ResourceARN] | @sh" | tr -d \')

for group in $tgroups; do
    aws elbv2 modify-target-group-attributes --target-group-arn $group --attributes Key=proxy_protocol_v2.enabled,Value=$2 > /dev/null
done
