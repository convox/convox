#!/bin/bash
set -ex -o pipefail

convox rack uninstall ${RACK_NAME}