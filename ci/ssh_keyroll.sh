#!/bin/bash
set -ex -o pipefail

convox switch ${RACK_NAME}

convox rack

convox instances keyroll > ~/keyroll.out

sed '0,/^Generated private key:$/d' ~/keyroll.out > ~/priv.key

cat ~/priv.key

instance=$(convox instances | awk '{print $1}' | sed -n 2p)

convox instances ssh ${instance} "echo hello" --key ~/priv.key | grep 'hello'
