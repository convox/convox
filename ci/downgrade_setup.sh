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
convox apps create httpd2
convox apps
convox apps | grep httpd2
convox apps info httpd2 | grep running
convox deploy -a httpd2
endpoint=$(convox api get /apps/httpd2/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
echo "ENDPOINT=${endpoint}" | convox env set -a httpd2
# postgres resource test
convox resources -a httpd2 | grep postgresdb
ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
convox exec $ps "/usr/scripts/db_insert.sh" -a httpd2
convox exec $ps "/usr/scripts/db_check.sh" -a httpd2
# redis resource test
convox resources -a httpd2 | grep rediscache
ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd2
