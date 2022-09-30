#!/bin/bash

convox resources -a httpd | grep postgresdb
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | grep web | head -n 1)
convox exec $ps "/usr/scripts/db_insert.sh" -a httpd
convox exec $ps "/usr/scripts/db_check.sh" -a httpd
convox resources export postgresdb -f /tmp/pdb.sql
convox resources import postgresdb -f /tmp/pdb.sql
