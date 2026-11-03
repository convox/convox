#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

sleep 120

wait_ready
