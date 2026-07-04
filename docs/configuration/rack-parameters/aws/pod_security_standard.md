---
title: "pod_security_standard"
description: "The pod_security_standard AWS rack parameter applies a Kubernetes Pod Security Standard, baseline or restricted, to every App namespace on the Rack."
slug: pod_security_standard
url: /configuration/rack-parameters/aws/pod_security_standard
---

# pod_security_standard

## Description

The `pod_security_standard` parameter applies a Kubernetes [Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/) to every App namespace on the Rack. When a standard is set, the Rack adds the `pod-security.kubernetes.io/<mode>: <standard>` label to each App namespace on that App's next deploy, and Kubernetes evaluates the App's pods against the standard in the mode selected by [pod_security_mode](/configuration/rack-parameters/aws/pod_security_mode).

- `baseline`: blocks the most common privilege escalations (privileged containers, host namespaces, host ports, hostPath mounts). A default Convox Service pod passes `baseline`.
- `restricted`: additionally requires pods to run as non-root, set a seccomp profile, disallow privilege escalation, and drop all capabilities. A default Convox Service pod does not meet `restricted`, so every Service needs a conforming [securityContext](/reference/primitives/app/service) in `convox.yml` before `restricted` can be enforced.

The standard is Rack-wide and evaluates every pod in each App namespace, including Services, Timers, and `convox run` processes. There is no per-App override, and Rack system namespaces are not labeled.

## Default Value

The default value is empty. With no standard set, App namespaces carry no Pod Security Admission label and admission behavior is unchanged, so upgrading an existing Rack to `3.24.10` changes nothing until you opt in.

## Setting the Parameter

```bash
$ convox rack params set pod_security_standard=baseline -r rackName
Updating parameters... OK
```

The Rack update reconfigures the Rack API, then the label reaches each App namespace on that App's next deploy. Roll out gradually: keep [pod_security_mode](/configuration/rack-parameters/aws/pod_security_mode) at `warn` (the default), review the admission warnings on deploys, move to `audit`, and set `enforce` only after workloads are confirmed clean.

Clear the value to disable Pod Security Admission labeling:

```bash
$ convox rack params set pod_security_standard= -r rackName
Updating parameters... OK
```

The label is removed from each App namespace on that App's next deploy.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

- **Validation:** must be `baseline`, `restricted`, or empty. The CLI rejects any other value.
- **Builds under `enforce`:** Builds run in a dedicated per-App build namespace so the standard does not reject build pods, and `convox build` and `convox deploy` continue to work without changes. See [pod_security_mode](/configuration/rack-parameters/aws/pod_security_mode) for details.
- **Leaving `enforce`:** after you revert the mode or clear the standard, each App namespace keeps its `enforce` label until that App's next deploy. Builds detect the remaining label and continue to run in the dedicated build namespace, so `convox build` and `convox deploy` work throughout the transition. `convox run` pods in that namespace follow the previous admission rules until the App deploys again. Running pods and `convox exec` are unaffected, since admission only evaluates new pods.
- The parameter configures the Rack API and creates no cloud resources. Moving the Rack to an earlier version removes it cleanly.

## See Also

- [pod_security_mode](/configuration/rack-parameters/aws/pod_security_mode)
- [Service securityContext](/reference/primitives/app/service)
- [seccomp_default_enabled](/configuration/rack-parameters/aws/seccomp_default_enabled)
