#!/bin/bash

function assert_run {
  run "$1" || { echo "failed"; exit 101; }
}

function run {
  echo "running: $*" >&2
  eval $*
}

root="$(cd $(dirname ${0:-})/..; pwd)"

. $root/ci/lib.sh

set -ex

provider=${PROVIDER:-$(convox api get /system | jq -r .provider)}
if [ -z "$provider" ] || [ "$provider" = "null" ]; then
  echo "could not determine the rack provider" >&2
  exit 1
fi
export provider

# cli
convox version

# rack
convox instances
convox rack
convox rack logs --no-follow --since 30m
convox rack logs --no-follow --since 30m | grep service/
convox rack ps | grep system

# registries: the aws provider reads an ecr registry's credentials as an access key pair, so aws racks pull a private docker hub image instead
convox registries
set +x
if [ "$provider" = "aws" ]; then
  registry=https://index.docker.io/v1/
  registry_user=convox
  registry_password=${CI_DOCKERHUB_TOKEN:?}
  registry_image=convox/ci-registry-test:1
else
  registry=${ECR_REGISTRY_HOST:?}
  registry_user=AWS
  registry_password=${ECR_REGISTRY_PASSWORD:?}
  registry_image=${ECR_REGISTRY_HOST}/convox-ci/registry-test:1
fi
convox registries add ${registry} ${registry_user} "${registry_password}"
set -x
convox registries | grep ${registry} | grep ${registry_user}
mkdir -p /tmp/ciregistry
printf 'FROM %s\n' ${registry_image} > /tmp/ciregistry/Dockerfile
printf 'services:\n  web:\n    build: .\n' > /tmp/ciregistry/convox.yml
convox apps create ciregistry
convox apps info ciregistry | grep running
(cd /tmp/ciregistry && convox build -a ciregistry -d ciregistry)
convox apps delete ciregistry
convox registries remove ${registry}
if convox registries | grep -q ${registry}; then
  exit 1
fi

# test files
sudo chmod -R +x $root/ci/test

# app (httpd)
cd $root/examples/httpd
convox apps create httpd
convox apps
convox apps | grep httpd
convox apps info httpd | grep running
release=$(convox build -a httpd -d cibuild --id)
[ -n "$release" ]
convox releases -a httpd | grep $release
build=$(convox api get /apps/httpd/builds | jq -r ".[0].id")
[ -n "$build" ]
convox builds -a httpd | grep $build
convox builds info $build -a httpd | grep $build
convox builds info $build -a httpd | grep cibuild
convox builds logs $build -a httpd | grep "load build context"
convox builds export $build -a httpd -f /tmp/build.tgz
releasei=$(convox builds import -a httpd -f /tmp/build.tgz --id)
[ -n "$releasei" ]
buildi=$(convox api get /apps/httpd/releases/$releasei | jq -r ".build")
[ -n "$buildi" ]
convox builds info $buildi -a httpd | grep cibuild
echo "FOO=bar" | convox env set -a httpd
convox env -a httpd | grep FOO | grep bar
convox env get FOO -a httpd | grep bar
convox env unset FOO -a httpd
if convox env -a httpd | grep -q "^FOO="; then
  exit 1
fi
releasee=$(convox env set FOO=bar -a httpd --id)
[ -n "$releasee" ]
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
sleep 30
ps=$(web_ps httpd)
convox ps info $ps -a httpd | grep $releaser
# assert the run command is going through the Dockerfile entrypoint
convox run web ls | grep entrypoint

$root/ci/test/app_scale.sh
$root/ci/test/app_htdocs.sh

convox scale web --count 1
convox deploy -a httpd
sleep 30
ps=$(web_ps httpd)
convox exec -a httpd $ps -- env | grep -q "^ONE_URL="
convox exec -a httpd $ps -- env | grep -q "^ONE_USER="
convox exec -a httpd $ps -- env | grep -q "^ONE_PASS="
convox exec -a httpd $ps -- env | grep -q "^ONE_HOST="
convox exec -a httpd $ps -- env | grep -q "^ONE_PORT="
convox exec -a httpd $ps -- env | grep -q "^ONE_NAME="
convox exec -a httpd $ps -- env | grep -q "^CUSTOM_DATABASE="

# test apps cancel
echo "FOO=not-bar" | convox env set -a httpd

cp Dockerfile Dockerfile.original # copy current Dockerfile
echo "COPY new-feature.html /usr/local/apache2/htdocs/index.html" >> Dockerfile
echo "ENTRYPOINT sleep 60 && httpd-foreground" >> Dockerfile
nohup convox deploy & # run deploy on background
deploy_pid=$!

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

echo "app is updating will cancel in 3 secs"
sleep 3

for i in {1..30}; do
  if convox apps cancel -a httpd | grep "OK"; then
    echo "app deployment canceled"
    break
  else
    echo "cancel attempt $i failed, retrying..."
    sleep 1
  fi
  if [ $i -eq 30 ]; then
    echo "Failed to cancel app deployment after 20 attempts"
    exit 1
  fi
done

endpoint=$(convox api get /apps/httpd/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"
echo "still returning the right content"

convox env -a httpd | grep "FOO" | grep "not-bar"
echo "env var is correctly set"

mv Dockerfile.original Dockerfile # replace the Dockerfile with the original copy

wait $deploy_pid || true

# timers
sleep 30
pids=()
$root/ci/test/timers.sh & pids+=($!)

# postgres resource test
$root/ci/test/resources_postgres.sh & pids+=($!)

# mariadb resource test
$root/ci/test/resources_mariadb.sh & pids+=($!)

# mysqldb resource test
$root/ci/test/resources_mysql.sh & pids+=($!)

# redis resource test
$root/ci/test/resources_redis.sh & pids+=($!)

# memcache resource test
$root/ci/test/resources_memcache.sh & pids+=($!)

# app (httpd2)
$root/ci/test/app_internal_communication.sh & pids+=($!)
wait_all "${pids[@]}"

# import/export test
convox apps create httpd-export
convox deploy -a httpd-export --manifest convox-export.yml
endpoint=$(convox api get /apps/httpd-export/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"

convox apps export -a httpd-export --file httpd-export.tgz

convox apps import -a httpd-import --file httpd-export.tgz
endpoint=$(convox api get /apps/httpd-import/services | jq -r '.[] | select(.name == "web") | .domain')
fetch https://$endpoint | grep "It works"

convox apps delete httpd-export
convox apps delete httpd-import

# app (gpu)
cd $root/examples/gpu
convox apps create gpu
convox apps
convox apps | grep gpu
convox apps info gpu | grep running
release=$(convox build -a gpu -d cibuild --id)
[ -n "$release" ]

# cleanup
convox apps delete gpu
convox apps delete httpd
convox apps delete httpd2
