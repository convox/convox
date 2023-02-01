#!/bin/bash
set -ex -o pipefail

redis-cli -u $REDIS_URL/0 SET convox rocks

sleep 5

redis-cli -u $REDIS_URL/0 GET convox
redis-cli -u $REDIS_URL/0 GET convox | grep rocks
