#!/bin/bash

root="$(cd $(dirname ${0:-})/..; pwd)"

. $root/ci/lib.sh

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
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/db_insert.sh" -a httpd2
convox exec $ps "/usr/scripts/db_check.sh" -a httpd2

# mariadb resource test
convox resources -a httpd2 | grep mariadb
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/mariadb_insert.sh" -a httpd2
convox exec $ps "/usr/scripts/mariadb_check.sh" -a httpd2

# mysqldb resource test
convox resources -a httpd2 | grep mysqldb
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/mariadb_insert.sh mysql" -a httpd2
convox exec $ps "/usr/scripts/mariadb_check.sh mysql" -a httpd2

# redis resource test
convox resources -a httpd2 | grep rediscache
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd2

# memcache resource test
convox resources -a httpd2 | grep memcache
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/memcache_check.sh" -a httpd2
