---
title: "seccomp_default_enabled"
description: "The seccomp_default_enabled AWS rack parameter applies the RuntimeDefault seccomp profile to the Convox system deployments, defaulting to false."
slug: seccomp_default_enabled
url: /configuration/rack-parameters/aws/seccomp_default_enabled
---

# seccomp_default_enabled

## Description

The `seccomp_default_enabled` parameter applies the `RuntimeDefault` seccomp profile to the following Convox system deployments: `api`, `atom`, `resolver`, `metrics-server`, and `metrics-scraper`. When enabled, each of these deployments receives a pod-level `securityContext.seccompProfile.type = RuntimeDefault`, restricting its containers to the curated syscall allowlist maintained by the node container runtime (containerd) instead of running unconfined. This aligns the system pods with the CIS EKS Benchmark and the Kubernetes Pod Security Standards "Restricted" seccomp requirement.

App pods and Build processes are unaffected. Seccomp for App Services remains configurable per Service through `securityContext` in `convox.yml`, and Build containers keep their existing build-time security context regardless of this parameter.

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

## Default Value

The default value is `false`. When disabled, no security context is added and the system pod specs are identical to previous Rack versions, so existing Racks see no change until you opt in.

## Setting the Parameter

```bash
$ convox rack params set seccomp_default_enabled=true -r rackName
Updating parameters... OK
```

Toggling the parameter in either direction performs a rolling restart of the five affected system deployments. App pods are not restarted and no nodes are replaced.

To return to the default behavior:

```bash
$ convox rack params set seccomp_default_enabled=false -r rackName
Updating parameters... OK
```

## Additional Information

- **Validation:** Must be `true` or `false`. The CLI rejects an empty value.
- **Reversible:** The parameter creates no new infrastructure. Disabling it restores the previous pod specs through the same in-place rolling update, and downgrading the Rack below `3.24.10` removes the parameter automatically.
- The nginx ingress controller already runs with the `RuntimeDefault` profile independently of this parameter.

## See Also

- [securityContext in convox.yml](/reference/primitives/app/service) for per-Service seccomp configuration on App pods
- [ebs_volume_encryption_enabled](/configuration/rack-parameters/aws/ebs_volume_encryption_enabled)
- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
