---
title: "Direct Kubernetes Access"
slug: direct-kubernetes-access
url: /management/direct-k8s-access
---
# Direct Kubernetes Access

Convox abstracts Kubernetes cluster management, but there are times when you need direct access to Kubernetes primitives and resources. Common use cases include debugging edge-case networking issues, examining underlying cloud infrastructure, understanding how components tie together, or installing a Kubernetes-native component or plugin directly.

> If you are troubleshooting a failed deployment, try [convox deploy-debug](/reference/cli/deploy-debug) first. It collects pod logs, events, and failure hints without requiring kubectl or kubeconfig.

Convox provides an API proxy to Kubernetes that runs on your Rack, allowing you to grant access to the underlying system while delegating per-user access to Kubernetes. This is easier and more manageable than providing direct Kubernetes credentials to every developer on your team.

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

In this way, you can produce a kubeconfig file for all the Racks you require and change which file your `kubectl` command references to control which Rack it connects to.

If you remove a user's access to your Convox organization, then they will also lose access to the underlying Kubernetes infrastructure, which is important from a security point of view.

## Access Scope

When you connect to the Kubernetes cluster via `convox rack kubeconfig`, your access is scoped to the cluster level. Each Convox app runs in its own Kubernetes namespace (named after the rack and app). You can view resources across namespaces using standard kubectl commands.

> Direct Kubernetes access bypasses Convox RBAC controls. Use caution when granting cluster access, as users can view and modify resources outside of the Convox abstraction layer.

## Useful Commands

### See the node metrics as reported by k8s

```sh
$ kubectl top node
```

### View the memory consumption and CPU time consumed by your services

```sh
$ kubectl top pod -l system=convox,app!=system --all-namespaces
```
