#!/bin/bash
set -ex -o pipefail

convox switch ${RACK_NAME}

convox rack

old_instances=$(convox instances | awk '{print $1}')

convox instances keyroll > ~/keyroll.out

sed '0,/^Generated private key:$/d' ~/keyroll.out > ~/priv.key

grep -q 'PRIVATE KEY' ~/priv.key

for i in $(seq 1 80); do
    instance=$(convox instances | awk '{print $1}' | sed -n 2p)
    if [[ $old_instances != *${instance}* ]]; then
      break
    fi
    if [ "$i" -eq 80 ]; then
      echo "no new instance after 20 minutes" >&2
      exit 1
    fi
    echo "waiting for new instances"
    sleep 15
done

instance=$(convox instances | awk '{print $1}' | sed -n 2p)

convox instances ssh ${instance} "echo hello" --key ~/priv.key | grep 'hello'
