#!/bin/bash

. "$(dirname "$0")/../lib.sh"

set -ex

convox resources -a httpd | grep mysqldb
ps=$(web_ps httpd)
convox exec $ps "/usr/scripts/mariadb_insert.sh mysql" -a httpd
convox exec $ps "/usr/scripts/mariadb_check.sh mysql" -a httpd
