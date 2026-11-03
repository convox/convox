#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

sleep 300

wait_ready
