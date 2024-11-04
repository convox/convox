MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "Running custom user data script"
cat <<'SCRIPT' > /opt/modify_kubelet.sh
#!/bin/bash

# Install jq if not already installed
yum install -y jq

# Path to the kubelet configuration file
KUBELET_CONFIG_FILE="/etc/kubernetes/kubelet/kubelet-config.json"

# Modify the registryPullQPS and registryBurst parameters in the kubelet config file
jq '.registryPullQPS = ${kubelet_registry_pull_qps} | .registryBurst = ${kubelet_registry_burst}' $KUBELET_CONFIG_FILE > /tmp/kubelet-config.json
mv -f /tmp/kubelet-config.json $KUBELET_CONFIG_FILE

# Reload the systemd daemon to apply the changes
systemctl daemon-reload

# Restart the kubelet service to apply the new configuration
systemctl restart kubelet
SCRIPT

chmod +x /opt/modify_kubelet.sh
/opt/modify_kubelet.sh

${user_data}

--==MYBOUNDARY==--