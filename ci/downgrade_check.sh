#!/bin/bash

function fetch {
  fetch_once $1 && sleep 5 && fetch_once $1
}

function fetch_once {
  curl -ks --connect-timeout 5 --max-time 3 --retry 100 --retry-max-time 600 --retry-connrefused $1
}

root="$(cd $(dirname ${0:-})/..; pwd)"

set -ex

provider=$(convox api get /system | jq -r .provider)

# cli
convox version

# rack
convox instances
convox rack

# app (httpd2)
cd $root/examples/httpd
convox apps
convox apps | grep httpd2
convox apps info httpd2 | grep running
endpoint=$(convox env get ENDPOINT -a httpd2)
fetch https://$endpoint | grep "It works"
# postgres resource test
ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
convox exec $ps "/usr/scripts/db_check.sh" -a httpd2
# redis resource test
convox resources -a httpd2 | grep rediscache
ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd2
#cleanup
convox apps delete httpd2
