#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

# a rack never moves to a lower minor, so the base is a previous patch of the same minor
base=${BASE_VERSION:-}
if [ -n "$base" ] && [ "${base%.*}" != "${VERSION%.*}" ]; then
  echo "::notice::base_version ${base} is not a ${VERSION%.*} release; downgrading to the previous ${VERSION%.*} release instead"
  base=""
fi
if [ -z "$base" ]; then
  base=$(previous_release ${VERSION} same-minor)
fi
if [ -z "$base" ]; then
  echo "::notice::no published ${VERSION%.*} release before ${VERSION}; downgrade skipped"
  exit 0
fi

convox rack update ${base} -r ${RACK_NAME} 2>&1 | redact

convox switch ${RACK_NAME}

wait_ready

version=$(convox rack | grep Version | awk -F '  +' '{print $2}')
if [ "${version}" != "${base}" ]; then
  exit 1;
fi
