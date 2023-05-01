#!/bin/bash
set -ex -o pipefail

convox switch ${RACK_NAME}

convox rack

old_instances=$(convox instances | awk '{print $1}')

convox instances keyroll > ~/keyroll.out

sed '0,/^Generated private key:$/d' ~/keyroll.out > ~/priv.key

cat ~/priv.key

while true
do
    instance=$(convox instances | awk '{print $1}' | sed -n 2p)
    if [[ $old_instances == *${instance}* ]]; then
      echo "waiting for new instances"
    else
      break
    fi
    sleep 15
done

instance=$(convox instances | awk '{print $1}' | sed -n 2p)

convox instances ssh ${instance} "echo hello" --key ~/priv.key | grep 'hello'
