#!/bin/bash
set -ex -o pipefail

psql -Atx $DB_URL -c 'SELECT COUNT(*) FROM data' | grep 5

echo "check is done"
