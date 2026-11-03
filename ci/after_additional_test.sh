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
convox apps
convox apps | grep httpd2
convox apps info httpd2 | grep running
endpoint=$(convox env get ENDPOINT -a httpd2)
fetch https://$endpoint | grep "It works"

# postgres resource test
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/db_check.sh" -a httpd2

# mariadb resource test
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/mariadb_check.sh" -a httpd2

# mysqldb resource test
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/mariadb_check.sh mysql" -a httpd2

# redis resource test
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd2

# memcache resource test
ps=$(web_ps httpd2)
convox exec $ps "/usr/scripts/memcache_check.sh" -a httpd2

#cleanup
convox apps delete httpd2
