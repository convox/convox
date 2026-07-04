---
title: "pod_security_mode"
description: "The pod_security_mode AWS rack parameter selects how Kubernetes applies the Pod Security Standard chosen by pod_security_standard: warn (default), audit, or enforce."
slug: pod_security_mode
url: /configuration/rack-parameters/aws/pod_security_mode
---

# pod_security_mode

## Description

The `pod_security_mode` parameter selects how Kubernetes applies the Pod Security Standard chosen by [pod_security_standard](/configuration/rack-parameters/aws/pod_security_standard) to App namespaces.

When a standard is set, the Rack labels each App namespace `pod-security.kubernetes.io/<mode>: <standard>` on that App's next deploy, and Kubernetes evaluates the App's pods against the standard in the selected mode:

- `warn` (default): the Kubernetes API returns warnings for non-conforming pods. Never blocks a deploy.
- `audit`: violations are recorded as annotations in the cluster audit log. Never blocks a deploy. On AWS Racks the control plane audit log is disabled by default, so viewing these annotations requires setting [eks_log_types](/configuration/rack-parameters/aws/eks_log_types) to include `audit`.
- `enforce`: non-conforming pods are rejected at admission. A deploy whose pods violate the standard fails.

This parameter has no effect while `pod_security_standard` is empty. The mode is Rack-wide; there is no per-App override.

## Default Value

The default value is `warn`. Because `pod_security_standard` defaults to empty, Racks that do not opt in apply no namespace label and see no change in behavior. Once a standard is set, `warn` surfaces violations without blocking any deploy.

## Setting the Parameter

```bash
$ convox rack params set pod_security_mode=audit -r rackName
Updating parameters... OK
```

Changing the value triggers a Rack update that reconfigures the Rack API. The new mode applies to each App namespace on that App's next deploy.

Move to `enforce` gradually: start at `warn`, review the admission warnings, move to `audit`, and set `enforce` only after workloads are confirmed clean. Setting `enforce` while a standard is active prints a CLI warning describing the impact.

If `enforce` blocks deploys, set `pod_security_mode=warn` to recover. The standard remains applied in warn mode, so it no longer rejects pods and Apps deploy again.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

- **Validation:** must be `warn`, `audit`, or `enforce`. The CLI rejects any other value, including an empty value.
- **Builds under enforce:** each App's Builds run in a dedicated per-App build namespace, so `convox build` and `convox deploy` continue to work without changes. While a Build runs, its build pod does not appear in `convox ps` for the App; Build output and logs are unaffected. In `warn` and `audit` modes, Builds run in the App namespace as before. Builds also route to the build namespace while an App namespace still carries an `enforce` label after leaving `enforce` mode, so Builds keep working through the transition.
- **`restricted` with `enforce`** rejects a default Convox Service pod. Every Service must set a conforming [securityContext](/reference/primitives/app/service) in `convox.yml` before enforcing `restricted`. `baseline` with `enforce` passes a default Service.
- The parameter configures the Rack API and creates no cloud resources. Moving the Rack to an earlier version removes it cleanly.

## See Also

- [pod_security_standard](/configuration/rack-parameters/aws/pod_security_standard)
- [eks_log_types](/configuration/rack-parameters/aws/eks_log_types)
- [Service securityContext](/reference/primitives/app/service)
- [system_readonly_rootfs_enabled](/configuration/rack-parameters/aws/system_readonly_rootfs_enabled)
