#!/bin/bash

. "$(dirname "$0")/../lib.sh"

set -ex

convox resources -a httpd | grep memcache
ps=$(web_ps httpd)
convox exec $ps "/usr/scripts/memcache_check.sh" -a httpd
