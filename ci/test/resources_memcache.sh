#!/bin/bash

set -ex

convox resources -a httpd | grep memcache
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
convox exec $ps "/usr/scripts/memcache_check.sh" -a httpd
