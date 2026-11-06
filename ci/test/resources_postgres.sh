#!/bin/bash

. "$(dirname "$0")/../lib.sh"

set -ex

convox resources -a httpd | grep postgresdb
ps=$(web_ps httpd)
convox exec $ps "/usr/scripts/db_insert.sh" -a httpd
convox exec $ps "/usr/scripts/db_check.sh" -a httpd
convox resources export postgresdb -f /tmp/pdb.sql
convox resources import postgresdb -f /tmp/pdb.sql
