#!/bin/bash
set -ex -o pipefail

redis-cli -u $REDIS_URL/0 -e SET convox rocks

sleep 5

redis-cli -u $REDIS_URL/0 -e GET convox
redis-cli -u $REDIS_URL/0 -e GET convox | grep rocks
