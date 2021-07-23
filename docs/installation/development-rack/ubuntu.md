# Ubuntu

## Initial Setup

> **_NOTE:_**:  Ubuntu 20.04 users will need to ensure that they are installing the latest version of Convox. See the 'Tips' sections below for more details.

## Docker

    $ sudo apt install docker.io

---

### Tips for Ubuntu 20.04 Users

> With Convox, local development racks depend on Docker as a container runtime. A local development rack deployment installs `microk8s` with kubernetes 1.13, which is the latest version of kubernetes that uses Docker. However, kubernetes 1.13 exposes [a bug with the underlying go library](https://github.com/kubernetes/kubernetes/blob/874f0559d9b358f87959ec0bb7645d9cb3d5f7ba/vendor/github.com/miekg/dns/clientconfig.go#L86). In short, the bug checks the `resolv.conf` file for DNS configuration options, and generates an error when the option length is exactly 8 characters.  Although this bug is fixed in subsequent versions of kubernetes, those versions of kubernetes do not use Docker, hence the reason why we have implemented the following installation workaround.

### Installation Workaround

In order to get the kubernetes steps that follow to work, a slight modification to your local `/etc/resolv.conf` file is required. `microk8s` leverages your local DNS configuration to configure DNS resolution in Kubernetes, and the following commands will provide you with a workaround to the problem described above:

Make a backup of your `/etc/resolv.conf` file:

    $ mv /etc/resolv.conf /etc/resolv.conf.orig

Important: Create a new configuration file by removing the `trust-ad` option from the options line in the original configuration file:

    $ cat /etc/resolv.conf.orig | sed 's/trust-ad//g' > /etc/resolv.conf.manually-configured

Next, create a symbolic link to the newly created `/etc/resolv.conf.manually-configured` file:

    $ ln -s /etc/resolv.conf.manually-configured /etc/resolv.conf

These steps allow your local DNS resolver to be manually configured.  Finally, restart the following processes to have this modification take affect:

    $ systemctl restart daemon-reload
    $ systemctl restart systemd-networkd systemd-resolved

Now, the Kubernetes steps below should work as expected.

### Workaround Script

For your convenience, we've provided the following script to assist you with the steps above.  Copy and paste this script into a file called `convox-workaround.sh`, then run this script as the `root` user with: `bash convox-workaround.sh` at the command line:

```
#!/usr/bin/env bash
# ------------------------------------------------------------------
#  Ubuntu 20.04 'workaround' script
# ------------------------------------------------------------------
RESOLV_CONF=/etc/resolv.conf
RESOLV_ORIG=/etc/resolv.conf.orig
RESOLV_MAN=/etc/resolv.conf.manually-configured

# Root execution Check
if [ "$EUID" -ne 0 ]; then 
  echo "ERROR: Please run this script as root. Exiting."
  exit
fi

# Add iptables rule
iptables -P FORWARD ACCEPT

# Make a backup copy of your /etc/resolv.conf file.
if [ -f "$RESOLV_CONF" ]; then
  mv $RESOLV_CONF $RESOLV_ORIG
else
  echo "ERROR: The $RESOLV_CONF file was not found. Exiting."
  exit
fi

# Create the manually configured version of the resolv.conf file.
if [ -f "$RESOLV_CONF_ORIG" ]; then
  cat $RESOLV_ORIG | sed 's/trust-ad//g' > $RESOLV_MAN
else
  echo "ERROR: The $RESOLV_CONF_ORIG file was not found. Exiting."
  exit
fi

# Create a symbolic link to the manually configured file.
if [ -f "$RESOLV_MAN" ]; then
  ln -s $RESOLV_MAN $RESOLV_CONF
else
  echo "ERROR: The $RESOLV_MAN file was not found. Exiting."
  exit
fi

# Restart DNS-related services
systemctl restart daemon-reload

if systemctl is-active --quiet systemd-networkd; then
  systemctl restart systemd-networkd
fi

if systemctl is-active --quiet systemd-resolved; then
  systemctl restart systemd-resolved
fi
```

---

## Kubernetes

    $ snap install microk8s --classic --channel=1.13/stable
    $ microk8s.start
    $ microk8s.enable dns storage
    $ mkdir -p ~/.kube
    $ microk8s.config > ~/.kube/config
    $ sudo snap restart microk8s
    $ sudo snap alias microk8s.kubectl kubectl

## Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

## Convox CLI

    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox

## Installation

> Make sure that your `kubectl` points at your local microk8s setup.  Ensure that if your `KUBECONFIG` env var is set, it is pointing at a config file that contains your local context.  By default as per the above instructions, your local config will be copied into `~/.kube/config`.

Install a local Rack named `dev`.

    $ convox rack install local dev

## DNS Setup

Set `*.convox` to be resolved by the local Rack's DNS server.

    $ sudo mkdir -p /usr/lib/systemd/resolved.conf.d
    $ sudo bash -c "printf '[Resolve]\nDNS=$(kubectl get service/resolver-external -n dev-system -o jsonpath="{.spec.clusterIP}")\nDomains=~convox' > /usr/lib/systemd/resolved.conf.d/convox.conf"
    $ systemctl daemon-reload
    $ systemctl restart systemd-networkd systemd-resolved

## CA Trust

To remove browser warnings about untrusted certificates for local applications
you can trust the Rack's CA certificate.

This certificate is generated on your local machine and is unique to your Rack.

    $ kubectl get secret/ca -n dev-system -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/ca
    $ sudo mv /tmp/ca /usr/local/share/ca-certificates/convox.crt
    $ sudo update-ca-certificates
    $ sudo snap restart microk8s
    $ sudo service docker restart

--- 

### Tips for Ubuntu 20.04 Users

Once you have completed the installation steps above, you will want to make sure that the `convox rack` commands are working by
executing the following command:
    
    $ convox rack -r dev

The output from this command should return a summary of your rack configuration similar to the output below:

    $ convox rack -r dev
    Name      dev
    Provider  local
    Router    router.dev.convox
    Status    running
    Version   3.0.49

If it doesn't, most likely you will receive a `504` error, as your local firewall rules are not allowing traffic to be forwarded to microk8s.
This can be resolved with the following command:

    $ iptables -P FORWARD ACCEPT

Issuing the `convox rack -r dev` command should now provide you with the appropriate output.

---