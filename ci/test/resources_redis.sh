#!/bin/bash
convox resources -a httpd | grep rediscache
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | grep web | head -n 1)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd
