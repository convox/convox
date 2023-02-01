#!/bin/bash

set -ex

convox scale web --count 2 --cpu 192 --memory 256 -a httpd
convox services -a httpd | grep web | grep 443:80 | grep $endpoint
endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
# waiting for web to scale up
i=0
while [ "$(convox ps -a httpd | grep web- | wc -l)" != "2" ]
do
    if [ $((i++)) -gt 10 ]; then
        exit 1
    fi
    echo "waiting for web to scale up..."
    convox ps -a httpd
    sleep 15
done
