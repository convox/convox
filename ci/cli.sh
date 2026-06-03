#!/bin/bash
set -ex -o pipefail

# resolve the latest release tag when VERSION is unset (same source install_last_release.sh uses)
if [ -z "${VERSION}" ]; then
  VERSION=$(curl -fsSL https://api.github.com/repos/convox/convox/releases/latest | jq -r '.tag_name')
fi
if [ -z "${VERSION}" ] || [ "${VERSION}" = "null" ]; then
  echo "could not resolve latest convox release tag" >&2
  exit 1
fi

# download appropriate cli version (-f so an HTTP error aborts instead of saving the error page as the binary)
curl -fL https://github.com/convox/convox/releases/download/${VERSION}/convox-linux -o /tmp/convox && \
  sudo mv /tmp/convox /usr/bin/convox && sudo chmod +x /usr/bin/convox
