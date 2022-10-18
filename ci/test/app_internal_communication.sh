#!/bin/bash

set -ex

convox apps create httpd2
convox apps
convox apps | grep httpd2
convox apps info httpd2 | grep running
convox deploy -a httpd2
endpoint=$(convox api get /apps/httpd2/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"

# test internal communication
rackname=$(convox rack | grep 'Name' | xargs | cut -d ' ' -f2 )

pshttpd=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | grep web | head -n 1)
pshttpd2=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | grep web | head -n 1)

sleep 10
convox exec $pshttpd "curl web.httpd2.$rackname.local" -a httpd | grep "It works"
convox exec $pshttpd2 "curl web.httpd.$rackname.local" -a httpd2 | grep "It works"
