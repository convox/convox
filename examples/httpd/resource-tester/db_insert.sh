#!/bin/bash
set -ex -o pipefail

psql -Atx $DB_URL -c 'CREATE TABLE data ( id int4 );'

counter=1
until [ $counter -gt 5 ]
do
  psql -Atx $DB_URL -c 'INSERT INTO data(id) VALUES (1);'
  ((counter++))
done

echo "data insertion is done"
