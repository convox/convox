---
title: "network_policy_enable"
description: "The network_policy_enable AWS rack parameter restricts which pods can open inbound connections to an App's pods, defaulting to false."
slug: network_policy_enable
url: /configuration/rack-parameters/aws/network_policy_enable
---

# network_policy_enable

## Description

The `network_policy_enable` parameter restricts which pods can open inbound connections to an App's pods. When enabled, the Rack maintains a Kubernetes NetworkPolicy named `convox-network-isolation` in every App namespace and turns on the AWS VPC CNI network policy engine so the policy is enforced.

The policy is ingress only and selects every pod in the namespace. Egress is untouched, so DNS, calls to third-party APIs, internet egress, and the egress policy that [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled) installs all behave as before.

What the parameter closes is the direct pod-to-pod path between App namespaces. Traffic that reaches an App through the Rack router is unaffected, because the router is an allowed source. That distinction decides which of your existing calls keep working, so read [How Apps Reach Each Other](#how-apps-reach-each-other) before enabling on a Rack that runs more than one App.

This parameter is available on AWS Racks only and requires Rack version `3.25.4` or later.

## Default Value

The default value is `false`. With the default no policy is created and the VPC CNI addon configuration is unchanged, so a Rack sees no change in behavior after updating.

## What the Policy Allows

An App pod accepts inbound connections from:

| Source | What runs there |
|--------|-----------------|
| Any pod in the App's own namespace | The App's Services, one-off Processes, Timers, and containerized Resources |
| `<rack>-system` | The Rack router, the Rack API, and the resolver |
| `kube-system` | Cluster DNS |
| `convox-monitoring` | Metrics scraping |
| Namespaces on the same Rack sharing the App namespace's tenant label | Apps grouped under that label |

Everything else is denied. The tenant-label source applies only where the App namespace carries the label, which an ordinary Rack does not set.

## How Apps Reach Each Other

Whether a cross-App call survives depends on the hostname it uses, because the two names Convox publishes for a Service resolve to different places.

| The caller uses | Resolves to | With the parameter on |
|-----------------|-------------|-----------------------|
| The hostname `convox services` reports, such as `web.myapp.0a1b2c3d4e5f.convox.cloud`, including the `convox.local` hostname of an `internal: true` Service | The Rack router | **Works.** The connection arrives from `<rack>-system`, an allowed source |
| `<service>.<app>.<rack>.local`, or a pod IP directly | The target Service's ClusterIP, or the pod itself | **Blocked.** The connection arrives carrying the calling pod's own IP, from a namespace that is not an allowed source |

Rewriting a blocked call to use the hostname `convox services` reports for that Service is usually the smallest change. See [Service Discovery](/configuration/service-discovery).

Blocked traffic is **dropped, not refused**. A caller waits until its own timeout rather than receiving a connection refused, so a client configured without a timeout stalls instead of failing fast.

## What Is Not Affected

- **Builds and deploys.** A build runs in its own namespace, which is not an App namespace and receives no policy. The policy is ingress only, so a build pod's outbound traffic is never restricted. A build pod cannot open a connection into an App pod, because its namespace is not an allowed source.
- **Certificate issuance.** The ACME HTTP solver pod is created inside the App's own namespace, so it is an allowed source even though `cert-manager` is not.
- **Containerized Resources.** A Postgres, MySQL, or Redis [Resource](/reference/primitives/app/resource) renders into its App's namespace and is addressed by same-namespace DNS, so it keeps working unchanged.
- **Egress of every kind**, including calls to the internet, to AWS APIs, and to other Racks.
- **`convox exec`, `convox logs`, `convox ps`**, and every other command served by the Rack API.

## Setting the Parameter

```bash
$ convox rack params set network_policy_enable=true -r rackName
Updating parameters... OK
```

The Rack runs a full reconcile pass when its API pod starts, so policies appear across every App namespace as soon as that pod restarts with the new setting. Namespaces for Apps created later are picked up on the next pass, at most two minutes on.

**The first enable is node-level churn, not a configuration-only change.** On a Rack not already running [pod_imds_block_enabled=true](/configuration/rack-parameters/aws/pod_imds_block_enabled), enabling this parameter changes the VPC CNI addon configuration, which updates the addon and rolls the `aws-node` DaemonSet on every node. Allow several minutes for the update. A Rack already running `pod_imds_block_enabled=true` has that configuration in place, so it sees no addon change and no roll.

Enforcement lags the policy object slightly: a freshly started pod is briefly reachable before its policy is programmed.

## Before You Enable It

- **An App that defines `balancers:` loses isolation across its whole namespace.** A [balancers](/configuration/load-balancers) entry creates a `type: LoadBalancer` Service with instance targets, so requests arrive from whichever node the load balancer selected, which no pod or namespace source can express. The Rack deletes the policy from any namespace holding a balancer Service. Adding a `balancers:` block to an App on a Rack with this parameter on therefore removes isolation from every Service in that App, not only from the balanced one.
- **Hand-written KEDA triggers that dial an in-cluster App endpoint stop working.** The `keda` namespace is not an allowed source. This applies to a trigger written under [`scale.autoscale.custom`](/configuration/scaling/autoscaling); every trigger Convox generates reads CPU or memory metrics or a Prometheus endpoint, so generated triggers are unaffected.
- **A Service running as an agent is covered like any other pod.** `agent: true` runs the Service as a DaemonSet and binds its ports on the node. The policy selects every pod in the namespace, agents included, so confirm what needs to reach an agent's host port first.

## Disabling

```bash
$ convox rack params set network_policy_enable=false -r rackName
Updating parameters... OK
```

Setting the parameter to `false` is what restores blocked traffic. The Rack deletes `convox-network-isolation` from every App namespace when its API pod restarts with the new setting. Enforcement can lag the deletion by a moment, so the first call after the revert may still fail where the next one succeeds.

Disabling does not roll `aws-node` a second time. The VPC CNI network policy engine stays enabled, which has no effect with no policies to enforce, and it stays enabled regardless while `pod_imds_block_enabled` is `true`.

## Downgrading Below 3.25.4

Set `network_policy_enable=false`, confirm traffic is restored, and let that update finish **before** downgrading a Rack below `3.25.4`.

Releases earlier than `3.25.4` do not manage these policies. A Rack that downgrades with the parameter still on leaves a `convox-network-isolation` policy in every App namespace with nothing maintaining it, and the VPC CNI keeps enforcing them. In that state enforcement no longer matches what the policy document permits: same-namespace traffic the policy explicitly admits has been observed blocked. Apps keep serving, because the router remains an allowed source, so the symptom is unexplained internal connectivity failures against a policy that reads as correct.

To recover, update the Rack back to `3.25.4` or later, set `network_policy_enable=false`, and let that apply finish. Setting `pod_imds_block_enabled=false` on the older version does not clear it: Terraform sends a null addon configuration, and EKS treats a null configuration as unmanaged rather than disabled, so the policy engine stays on.

## Additional Information

- **Validation:** must be `true` or `false`. Any other value is rejected.
- AWS only. On GCP, Azure, DigitalOcean, Metal, and Local Racks the CLI rejects the name as an unknown parameter.
- Setting it requires a `convox` CLI at `3.25.4` or newer. An older CLI rejects the name, so run [`sudo convox update`](/reference/cli/update) first.
- On a Rack earlier than `3.25.4` the value is accepted and then removed by parameter reconciliation before the apply runs, so no policy is created. See [Pre-Apply Reconciliation](/management/cli-rack-management#pre-apply-reconciliation).
- `convox rack params` lists stored values only, so `network_policy_enable` does not appear in its output until you set it.
- The parameter belongs to the `security` parameter group, so `convox rack params -g security` surfaces it once set.
- It shares one VPC CNI addon configuration with [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled). The two are independent and can be set together in a single command.
- Edits made by hand to a `convox-network-isolation` policy are reverted on the next reconcile pass.

## See Also

- [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled) for blocking App pod egress to IMDS
- [Service Discovery](/configuration/service-discovery) for the hostnames Convox publishes for a Service
- [Load Balancers](/configuration/load-balancers) for the `balancers:` block and dedicated load balancers
- [Autoscaling](/configuration/scaling/autoscaling) for KEDA triggers, including `scale.autoscale.custom`
- [pod_security_standard](/configuration/rack-parameters/aws/pod_security_standard) for Pod Security Admission on App namespaces
