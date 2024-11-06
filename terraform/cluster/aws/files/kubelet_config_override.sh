MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
exec > >(tee /var/log/user-data.log | logger -t user-data -s 2>/dev/console) 2>&1
echo "Running custom user data script"
cat <<'SCRIPT' > /opt/modify_kubelet.sh
#!/bin/bash
# Enable logging for debugging purposes
exec > >(tee /var/log/user-data.log | logger -t user-data -s 2>/dev/console) 2>&1

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

if [ "${disable_kernel_mitigations}" == "true" ]; then
    echo "Disabling kernel mitigations"

    mitigation_param=" mitigations=off"
    cp /etc/default/grub /etc/default/grub.bak
    current_cmdline=$(grep -Po 'GRUB_CMDLINE_LINUX_DEFAULT=.*(?=")' /etc/default/grub)
    new_cmdline="$current_cmdline$mitigation_param"
    sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=.*$/$new_cmdline\"/" /etc/default/grub
    grub2-mkconfig -o /boot/grub2/grub.cfg
    shutdown -r +1 "Rebooting to apply kernel mitigation changes after 1 minute"
fi

SCRIPT

chmod +x /opt/modify_kubelet.sh
/opt/modify_kubelet.sh

--==MYBOUNDARY==--