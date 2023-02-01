#!/bin/bash

set -ex

convox resources -a httpd | grep mariadb
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
convox exec $ps "/usr/scripts/mariadb_insert.sh" -a httpd
convox exec $ps "/usr/scripts/mariadb_check.sh" -a httpd
