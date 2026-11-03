#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

if [ -z "${VERSION}" ]; then
  VERSION=$(latest_release)
fi

# download appropriate cli version (-f so an HTTP error aborts instead of saving the error page as the binary)
curl -fL https://github.com/convox/convox/releases/download/${VERSION}/convox-linux -o /tmp/convox && \
  sudo mv /tmp/convox /usr/bin/convox && sudo chmod +x /usr/bin/convox
