#!/bin/bash

. "$(dirname "$0")/../lib.sh"

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

pshttpd=$(web_ps httpd)
pshttpd2=$(web_ps httpd2)

sleep 10
convox exec $pshttpd "curl web.httpd2.$rackname.local" -a httpd | grep "It works"
convox exec $pshttpd2 "curl web.httpd.$rackname.local" -a httpd2 | grep "It works"
