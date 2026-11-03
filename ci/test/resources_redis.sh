#!/bin/bash

. "$(dirname "$0")/../lib.sh"

set -ex

convox resources -a httpd | grep rediscache
ps=$(web_ps httpd)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd
