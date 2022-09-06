#!/bin/bash

function assert_run {
  run "$1" || { echo "failed"; exit 101; }
}

function fetch {
  fetch_once $1 && sleep 5 && fetch_once $1
}

function fetch_once {
  curl -ks --connect-timeout 5 --max-time 3 --retry 100 --retry-max-time 600 --retry-connrefused $1
}

function run {
  echo "running: $*" >&2
  eval $*
}

root="$(cd $(dirname ${0:-})/..; pwd)"

set -ex

provider=$(convox api get /system | jq -r .provider)

# cli
convox version

# rack
convox instances
convox rack
convox rack logs --no-follow --since 30m
convox rack logs --no-follow --since 30m | grep service/
convox rack ps | grep system

# downgrade persistance testing
if [ "$1" == "downgrade-setup" ]; then
    # app (httpd2)
    cd $root/examples/httpd
    convox apps create httpd2
    convox apps
    convox apps | grep httpd2
    convox apps info httpd2 | grep running
    convox deploy -a httpd2
    endpoint=$(convox api get /apps/httpd2/services | jq -r '.[] | select(.name == "web") | .domain')
    fetch https://$endpoint | grep "It works"
    echo "ENDPOINT=${endpoint}" | convox env set -a httpd2
    # postgres resource test
    convox resources -a httpd2 | grep postgresdb
    ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
    convox exec $ps "/usr/scripts/db_insert.sh" -a httpd2
    convox exec $ps "/usr/scripts/db_check.sh" -a httpd2
    # redis resource test
    convox resources -a httpd2 | grep rediscache
    ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
    convox exec $ps "/usr/scripts/redis_check.sh" -a httpd2

    # downgrate-setup runs before the downgrading, so no need to test others functionality
    # they are covered by other ci test
    exit 0

elif [ "$1" == "downgrade-check" ]; then
    convox apps
    convox apps | grep httpd2
    convox apps info httpd2 | grep running
    endpoint=$(convox env get ENDPOINT -a httpd2)
    fetch https://$endpoint | grep "It works"
    # postgres resource test
    ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
    convox exec $ps "/usr/scripts/db_check.sh" -a httpd2
    # redis resource test
    convox resources -a httpd2 | grep rediscache
    ps=$(convox api get /apps/httpd2/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
    convox exec $ps "/usr/scripts/redis_check.sh" -a httpd2
    #cleanup
    convox apps delete httpd2
fi

# registries
convox registries
convox registries add quay.io convox+ci 6D5CJVRM5P3L24OG4AWOYGCDRJLPL0PFQAENZYJ1KGE040YDUGPYKOZYNWFTE5CV
convox registries | grep quay.io | grep convox+ci
convox registries remove quay.io
convox registries | grep -v quay.io

# app (httpd)
cd $root/examples/httpd
convox apps create httpd
convox apps
convox apps | grep httpd
convox apps info httpd | grep running
release=$(convox build -a httpd -d cibuild --id) && [ -n "$release" ]
convox releases -a httpd | grep $release
build=$(convox api get /apps/httpd/builds | jq -r ".[0].id") && [ -n "$build" ]
convox builds -a httpd | grep $build
convox builds info $build -a httpd | grep $build
convox builds info $build -a httpd | grep cibuild
convox builds logs $build -a httpd | grep "Running: docker push"
convox builds export $build -a httpd -f /tmp/build.tgz
releasei=$(convox builds import -a httpd -f /tmp/build.tgz --id) && [ -n "$releasei" ]
buildi=$(convox api get /apps/httpd/releases/$releasei | jq -r ".build") && [ -n "$buildi" ]
convox builds info $buildi -a httpd | grep cibuild
echo "FOO=bar" | convox env set -a httpd
convox env -a httpd | grep FOO | grep bar
convox env get FOO -a httpd | grep bar
convox env unset FOO -a httpd
convox env -a httpd | grep -v FOO
releasee=$(convox env set FOO=bar -a httpd --id) && [ -n "$releasee" ]
convox env get FOO -a httpd | grep bar
convox releases -a httpd | grep $releasee
convox releases info $releasee -a httpd | grep FOO
convox releases manifest $releasee -a httpd | grep "build: ."
convox releases promote $release -a httpd
endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
sleep 30
convox rack ps -a | grep httpd
convox logs -a httpd --no-follow --since 10m
convox logs -a httpd --no-follow --since 10m | grep service/web
releaser=$(convox releases rollback $release -a httpd --id)
convox ps -a httpd | grep $releaser
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
convox ps info $ps -a httpd | grep $releaser
convox scale web --count 2 --cpu 192 --memory 256 -a httpd
convox services -a httpd | grep web | grep 443:80 | grep $endpoint
endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
# waiting for web to scale up
i=0
while [ "$(convox ps -a httpd | grep web | wc -l)" != "2" ]
do
    if [ $((i++)) -gt 50 ]; then
        exit 1
    fi
    echo "waiting for web to scale up..."
    convox ps -a httpd
    sleep 15
done
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | head -n 1)
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
convox ps -a httpd | grep -v $ps
convox scale web --count 1
convox deploy -a httpd

# timers
sleep 30

case $provider in
   gcp)
      convox logs -a httpd --no-follow --since 1m | grep timer/example/timer-example | grep "Hello Timer"
      ;;
   *)
      convox logs -a httpd --no-follow --since 1m | grep service/web/timer-example | grep "Hello Timer"
      ;;
esac

# postgres resource test
convox resources -a httpd | grep postgresdb
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
convox exec $ps "/usr/scripts/db_insert.sh" -a httpd
convox exec $ps "/usr/scripts/db_check.sh" -a httpd
convox resources export postgresdb -f /tmp/pdb.sql
convox resources import postgresdb -f /tmp/pdb.sql

# redis resource test
convox resources -a httpd | grep rediscache
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "resource-tester")|.id' | head -n 1)
convox exec $ps "/usr/scripts/redis_check.sh" -a httpd

# cleanup
convox apps delete httpd
