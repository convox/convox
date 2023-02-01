#!/bin/bash
set -ex -o pipefail

dburl=$MARIA_URL
if [ $1 == "mysql" ]; then
  dburl=$MYSQL_URL
fi

proto="$(echo $dburl | grep :// | sed -e's,^\(.*://\).*,\1,g')"
# remove the protocol
url="$(echo ${dburl/$proto/})"
# extract the user (if any)
userpass="$(echo $url | grep @ | cut -d@ -f1)"
pass="$(echo $userpass | grep : | cut -d: -f2)"
if [ -n "$pass" ]; then
  user="$(echo $userpass | grep : | cut -d: -f1)"
else
    user=$userpass
fi

# extract the host
host_with_port="$(echo ${url/$userpass@/} | cut -d/ -f1)"
# by request - try to extract the port
port="$(echo $host_with_port | sed -e 's,^.*:,:,g' -e 's,.*:\([0-9]*\).*,\1,g' -e 's,[^0-9],,g')"
host="$(echo ${host_with_port} | cut -d: -f1)"

dbname="$(echo $url | grep / | cut -d/ -f2-)"

mariadb -h${host} -P${port} -u${user} -p${pass} -D${dbname} -e 'SELECT COUNT(*) FROM data' | grep 5

echo "check is done"
