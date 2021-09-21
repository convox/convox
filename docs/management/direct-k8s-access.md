---
title: "Direct Kubernetes Access"
draft: false
slug: Direct Kubernetes Access
url: /management/direct-k8s-access
---
# Direct Kubernetes Access

Convox makes managing and operating your Kubernetes cluster extremely easy and simple by abstracting a lot of the unnecessary complexities away.  We know, however, that there may be times that you will want to dive deeper and access the Kubernetes primitives and resources yourself.  This could be deep debugging of an edge-case networking issue, examination of an underlying cloud infrastructure issue, gaining a deeper understanding of how everything ties together, or simply to install a Kubernetes native component or plugin directly.

Convox provides an API proxy to Kubernetes that runs on your Rack which allows you to grant availability to the underlying system, whilst delegating per-user access to Kubernetes.  This is actually a lot easier and more manageable than providing direct Kubernetes credentials to the developers in your team!

## Configure kubectl to Point at Your Rack

Convox allows you to securely connect your `kubectl` to your Convox created Kubernetes cluster by exporting a [kubeconfig](/reference/cli/rack#rack-kubeconfig) that will connect you to a Kubernetes API Proxy running inside your Rack. This allows you to use `kubectl` without directly exposing the credentials for your Kubernetes cluster. For example if your Rack is called `myrack` you could point your local `kubectl` to your Rack cluster as follows

```sh
$ convox switch myrack
$ convox rack kubeconfig > $HOME/.kube/myrack-config
$ export KUBECONFIG=$HOME/.kube/myrack-config
or
$ kubectl get pods --namespace=myrack-system --kubeconfig=$HOME/.kube/myrack-config
```

This will export the proxy configuration to a temporary file and then point your local `kubectl` environment at that location so you can connect to your Rack's cluster. You will need to perform this step before you can execute any `kubectl` commands against your cluster.

> By default, `kubectl` looks for a file named `config` in the `$HOME/.kube` directory. You can specify other kubeconfig files by setting the `KUBECONFIG` environment variable or by setting the `--kubeconfig` flag.

In this way, you can produce a kubeconfig file for all the Racks you require and simply change which file your `kubectl` command refers to to change which Rack it talks to.

If you remove a user's access to your Convox organization, then they will also lose access to the underlying Kubernetes infrastructure, which is important from a security point of view.

## Useful Commands

### See the node metrics as reported by k8s

```sh
$ kubectl top node
```

### View the memory consumption and CPU time consumed by your services

```sh
$ kubectl top pod -l system=convox,app!=system --all-namespaces
```
