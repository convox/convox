# Ubuntu

## Initial Setup

> **_NOTE:_**:  Ubuntu 20.04 users will need ensure that they are installing the latest version of convox. See more 'Tips' sections below for more details.

### Docker

    $ sudo apt install docker.io

---

### Tips for Ubuntu 20.04 Users

As an Unbuntu 20.04 user, in order for the Kubernetes setup below to work as expected, a slight modification to your local `/etc/resolv.conf` file is required. `microk8s` leverages your local DNS configuration to configure DNS resolution in Kubernetes, and the following commands will resolve a bug that is encountered with Kubernetes version 1.13:

Make a backup of your `/etc/resolv.conf` file:

    $ mv /etc/resolv.conf /etc/resolv.conf.orig

Next, make a copy of the original file and name is as follows:

    $ cp /etc/resolv.conf.orig /etc/resolv.conf.manually-configured

Modify the newly created `/etc/resolv.conf.manually-configured` file by removing the `trust-ad` option from the options line.

Next, create a symbolic link to the newly created `/etc/resolv.conf.manually-configured` file:

    $ ln -s /etc/resolv.conf.manually-configured /etc/resolv.conf

These steps allow your local DNS resolver to be manually configured.  Finally, restart the following processes to have this modification take affect:

    $ systemctl restart daemon-reload
    $ systemctl restart systemd-networkd systemd-resolved

---

### Kubernetes

    $ snap install microk8s --classic --channel=1.13/stable
    $ microk8s.start
    $ microk8s.enable dns storage
    $ mkdir -p ~/.kube
    $ microk8s.config > ~/.kube/config
    $ sudo snap restart microk8s
    $ sudo snap alias microk8s.kubectl kubectl

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI

    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox

## Installation

> Make sure that your `kubectl` points at your local microk8s setup.  Ensure that if your `KUBECONFIG` env var is set, it is pointing at a config file that contains your local context.  By default as per the above instructions, your local config will be copied into `~/.kube/config`.

Install a local Rack named `dev`.

    $ convox rack install local dev

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

If it doesn't, most likely you're recieved a 504 error, as your local firewall rules are not allowing traffic to be forwarded to microk8s.
This can be resolved with the following command:

  $ iptables -P FORWARD ACCEPT

Issuing the `convox rack -r dev` command should now provide you with the appropriate output.

---

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