#!/bin/bash

convox resources -a httpd | grep mysqldb
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
convox exec $ps "/usr/scripts/mariadb_insert.sh mysql" -a httpd
convox exec $ps "/usr/scripts/mariadb_check.sh mysql" -a httpd
