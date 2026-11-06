---
title: "system_readonly_rootfs_enabled"
description: "The system_readonly_rootfs_enabled AWS rack parameter runs the Convox system containers (api, atom, and resolver) with a read-only root filesystem."
slug: system_readonly_rootfs_enabled
url: /configuration/rack-parameters/aws/system_readonly_rootfs_enabled
---

# system_readonly_rootfs_enabled

## Description

The `system_readonly_rootfs_enabled` parameter runs the Convox system containers (api, atom, and resolver) with `readOnlyRootFilesystem: true`, so the contents of those container images cannot be modified at runtime. This is the read-only root filesystem hardening recommended by the EKS Best Practices Guide and standard container hardening benchmarks, applied to the Rack control plane.

When enabled, the Rack also provisions the writable scratch space each container needs:

| Container | Read-only root filesystem | Writable scratch |
|-----------|---------------------------|------------------|
| api | yes | `emptyDir` volumes at `/tmp` and `/var/tmp`, with `HOME` set to `/tmp` |
| atom | yes | `emptyDir` volume at `/tmp`, with `HOME` set to `/tmp` |
| resolver | yes | none required |

This parameter affects only the Convox system Deployments. The ingress router, Fluentd, and App containers are not affected. Control the read-only root filesystem setting for your own Services with `securityContext.readOnlyRootFilesystem` in `convox.yml`.

## Default Value

The default value is `false`. At the default, this parameter adds nothing to the api, atom, and resolver pod templates, so leaving it unset causes no pod template changes or restarts attributable to this parameter. A Rack version update still performs its normal rolling update of the system pods.

## Setting the Parameter

```bash
$ convox rack params set system_readonly_rootfs_enabled=true -r rackName
Updating parameters... OK
```

Enabling changes the api, atom, and resolver pod templates, so each of those Deployments performs one rolling update. To restore the previous behavior:

```bash
$ convox rack params set system_readonly_rootfs_enabled=false -r rackName
Updating parameters... OK
```

Disabling rolls the same Deployments back to writable root filesystems. The only objects this parameter creates are pod-local `emptyDir` volumes, so disabling leaves nothing behind.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

- **Validation:** boolean. The CLI rejects non-boolean values with an error stating the param must be `'true'` or `'false'`.
- The `emptyDir` scratch volumes are pod-local and empty on every pod start, which matches how the system containers use `/tmp`.

## See Also

- [Service securityContext](/reference/primitives/app/service) for read-only root filesystems on your own Services
- [ebs_volume_encryption_enabled](/configuration/rack-parameters/aws/ebs_volume_encryption_enabled)
- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
