#!/bin/bash
set -ex -o pipefail

proto="$(echo $MEMCACHE_URL | grep :// | sed -e's,^\(.*://\).*,\1,g')"
# remove the protocol
url="$(echo ${MEMCACHE_URL/$proto/})"

# extract the host
host="$(echo ${url} | cut -d: -f1)"
# by request - try to extract the port
port="$(echo ${url} | cut -d: -f2)"

printf "set test 0 300 6\r\nconvox\r\n" | ncat $host $port | grep STORED

printf "get test\r\n" | ncat $host $port

printf "get test\r\n" | ncat $host $port | grep convox
