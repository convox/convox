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

# apps
cd $root/examples/httpd
# app (httpd)
convox apps create httpd
convox apps
convox apps | grep httpd
convox apps info httpd | grep running
convox deploy -a httpd
endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"

# app (httpd2)
convox apps create httpd2
convox apps
convox apps | grep httpd2
convox apps info httpd2 | grep running
convox deploy -a httpd2
endpoint=$(convox api get /apps/httpd2/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"

# test
rackname=$(convox rack | grep 'Name' | xargs | cut -d ' ' -f2 )

pshttpd=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
pshttpd2=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)

sleep 10
convox exec $pshttpd "curl web.httpd2.$rackname.local" -a httpd | grep "It works"
convox exec $pshttpd2 "curl web.httpd.$rackname.local" -a httpd2 | grep "It works"

#cleanup
convox apps delete httpd
convox apps delete httpd2
