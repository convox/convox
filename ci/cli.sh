#!/bin/bash
set -ex -o pipefail

# download appropriate cli version
curl -L https://github.com/convox/convox/releases/download/${VERSION}/convox-linux -o /tmp/convox && \
  sudo mv /tmp/convox /usr/bin/convox && sudo chmod +x /usr/bin/convox