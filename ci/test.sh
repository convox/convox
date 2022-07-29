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
convox releases manifest $releasee -a httpd | grep "image: httpd"
convox releases promote $release -a httpd
endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
sleep 30
convox rack ps -a | grep httpd
convox logs -a httpd --no-follow --since 10m | grep service/web
releaser=$(convox releases rollback $release -a httpd --id)
convox ps -a httpd | grep $releaser
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running")|.id' | head -n 1)
convox ps info $ps -a httpd | grep $releaser
convox scale web --count 2 --cpu 192 --memory 256 -a httpd
convox services -a httpd | grep web | grep 443:80 | grep $endpoint
endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
convox ps -a httpd | grep web | wc -l | grep 2
ps=$(convox api get /apps/httpd/processes | jq -r '.[]|select(.status=="running")|.id' | head -n 1)
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
convox deploy -a httpd

# cleanup
convox apps delete httpd
