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

echo "get system"
convox api get /system

provider=$(convox api get /system | jq -r .provider)

# cli
convox version

# rack
convox instances
convox rack
convox rack logs --no-follow --since 30m
convox rack logs --no-follow --since 30m | grep service/
convox rack ps | grep system

# registries
convox registries
convox registries add quay.io convox+ci 6D5CJVRM5P3L24OG4AWOYGCDRJLPL0PFQAENZYJ1KGE040YDUGPYKOZYNWFTE5CV
convox registries | grep quay.io | grep convox+ci
convox registries remove quay.io
convox registries | grep -v quay.io

# test files
sudo chmod -R +x $root/ci/test

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
convox builds logs $build -a httpd | grep "pushing manifest for"
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
convox logs -a httpd --no-follow --since 1m
convox logs -a httpd --no-follow --since 1m | grep service/web
releaser=$(convox releases rollback $release -a httpd --id)
convox ps -a httpd | grep $releaser
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running" and .name == "web")|.id' | grep web | head -n 1)
convox ps info $ps -a httpd | grep $releaser

$root/ci/test/app_scale.sh &
$root/ci/test/app_htdocs.sh &

wait

convox scale web --count 1
convox deploy -a httpd

# test apps cancel
echo "FOO=not-bar" | convox env set -a httpd

cp Dockerfile Dockerfile.original # copy current Dockerfile
echo "COPY new-feature.html /usr/local/apache2/htdocs/index.html" >> Dockerfile
echo "ENTRYPOINT sleep 60 && httpd-foreground" >> Dockerfile
nohup convox deploy & # run deploy on background

i=0
while [ "$(convox apps info -a httpd | grep updating | wc -l)" != "1" ]
do
  # exit if takes more than 60 seconds/times
  if [ $((i++)) -gt 60 ]; then
    exit 1
  fi
  echo "waiting for web to be marked as updating..."
  sleep 1
done

echo "app is updating will cancel in 10 secs"
sleep 10

convox apps cancel -a httpd | grep "OK"
echo "app deployment canceled"

endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
echo "still returning the right content"

convox env -a httpd | grep "FOO" | grep "not-bar"
echo "env var is correctly set"

mv Dockerfile.original Dockerfile # replace the Dockerfile with the original copy

# timers
sleep 30
$root/ci/test/timers.sh &

# postgres resource test
$root/ci/test/resources_postgres.sh &

# mariadb resource test
$root/ci/test/resources_mariadb.sh &

# mysqldb resource test
$root/ci/test/resources_mysql.sh &

# redis resource test
$root/ci/test/resources_redis.sh &

# memcache resource test
$root/ci/test/resources_memcache.sh &

# app (httpd2)
$root/ci/test/app_internal_communication.sh &
wait

# cleanup
convox apps delete httpd
convox apps delete httpd2
