#!/bin/bash

. "$(dirname "$0")/../lib.sh"

set -ex

ps=$(web_ps httpd)
convox exec $ps "ls -la" -a httpd | grep htdocs
cat /dev/null | convox exec $ps 'sh -c "sleep 2; echo test"' -a httpd | grep test
convox run web "ls -la" -a httpd | grep htdocs
cat /dev/null | convox run web 'sh -c "sleep 2; echo test"' -a httpd | grep test
echo foo > /tmp/file
convox cp /tmp/file $ps:/file -a httpd
convox exec $ps "cat /file" -a httpd | grep foo
mkdir -p /tmp/dir
echo foo > /tmp/dir/file
convox cp /tmp/dir $ps:/dir -a httpd
convox exec $ps "cat /dir/file" -a httpd | grep foo
convox cp $ps:/dir /tmp/dir2 -a httpd
cat /tmp/dir2/file | grep foo
convox cp $ps:/file /tmp/file2 -a httpd
cat /tmp/file2 | grep foo
convox ps stop $ps -a httpd
i=0
while convox ps -a httpd | grep -q $ps; do
  if [ $((i++)) -gt 12 ]; then
    exit 1
  fi
  sleep 5
done
