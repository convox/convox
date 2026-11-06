#!/bin/bash

. "$(dirname "$0")/../lib.sh"

set -ex

convox resources -a httpd | grep mariadb
ps=$(web_ps httpd)
convox exec $ps "/usr/scripts/mariadb_insert.sh" -a httpd
convox exec $ps "/usr/scripts/mariadb_check.sh" -a httpd
