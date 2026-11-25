MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==CONVOXGVISOR=="

%{ if user_data != "" || user_data_script_file != "" ~}
--==CONVOXGVISOR==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "USER PROVIDED USER DATA SCRIPT"
${user_data}

${user_data_script_file}

%{ endif ~}
--==CONVOXGVISOR==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
# Installs the pinned gVisor release for the gvisor RuntimeClass. cloud-final runs this before
# nodeadm-run starts the kubelet, so the runtime is in place before the node takes pods. If the
# install fails the node still joins, and gvisor pods fail to start instead of running without
# gVisor.
set -euo pipefail
echo "CONVOX GVISOR INSTALL ${gvisor_release}"

case "$(uname -m)" in
  x86_64) arch=x86_64 sha512="${gvisor_sha512_x86_64}" ;;
  aarch64) arch=aarch64 sha512="${gvisor_sha512_aarch64}" ;;
  *) echo "gvisor: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
url="https://storage.googleapis.com/gvisor/releases/release/${gvisor_release}/$arch/gvisor.tar.bz2"
for attempt in 1 2 3 4 5; do
  if curl -fsSL --retry 3 --connect-timeout 10 -o "$work/gvisor.tar.bz2" "$url"; then
    break
  fi
  [ "$attempt" = 5 ] && { echo "gvisor: download failed: $url" >&2; exit 1; }
  sleep $((attempt * 5))
done
echo "$sha512  $work/gvisor.tar.bz2" | sha512sum -c -
tar -xjf "$work/gvisor.tar.bz2" -C "$work" runsc containerd-shim-runsc-v1
install -m 0755 "$work/runsc" "$work/containerd-shim-runsc-v1" /usr/local/bin/

major="$(containerd --version | sed -E 's/.* v?([0-9]+)\..*/\1/')"
if [ "$major" -ge 2 ]; then
  version=3 plugin="io.containerd.cri.v1.runtime"
else
  version=2 plugin="io.containerd.grpc.v1.cri"
fi
mkdir -p /etc/containerd/config.d
cat > /etc/containerd/config.d/90-convox-gvisor.toml <<EOF
version = $version

[plugins."$plugin".containerd.runtimes.runsc]
  runtime_type = "io.containerd.runsc.v1"
EOF

if systemctl is-active --quiet containerd; then
  systemctl restart containerd
fi
echo "gvisor: $(/usr/local/bin/runsc --version | head -1) registered with containerd v$major"

--==CONVOXGVISOR==
Content-Type: application/node.eks.aws

apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  containerd:
    config: |
      imports = ["/etc/containerd/config.d/*.toml"]
%{ if fast_image_pull ~}
  featureGates:
    FastImagePull: true
%{ endif ~}
%{ if kubelet_registry_set ~}
  kubelet:
    config:
      registryPullQPS: ${kubelet_registry_pull_qps}
      registryBurst: ${kubelet_registry_burst}
%{ endif ~}

--==CONVOXGVISOR==--
