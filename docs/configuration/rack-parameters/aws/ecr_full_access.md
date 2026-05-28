---
title: "ecr_full_access"
slug: ecr_full_access
url: /configuration/rack-parameters/aws/ecr_full_access
---

# ecr_full_access

## Description

The `ecr_full_access` parameter re-attaches the AWS managed `AmazonEC2ContainerRegistryFullAccess` policy to the Rack API IAM role. This restores the pre-3.24.6 ECR permission scope for racks where the tighter scoped inline policy introduced in 3.24.6 is too restrictive.

Convox 3.24.6 replaced the broad `AmazonEC2ContainerRegistryFullAccess` managed policy with a scoped inline policy limited to `${rack_name}/*` and `${rack_name}-*` ECR repositories. This is the correct scope for most racks, but it blocks CI pipelines that push pre-built images to bare-named ECR repos in the same account or reference them as `--cache-from` sources during `convox build`.

For finer-grained control over which additional ECR repositories the Rack API role can access, use [`ecr_additional_policy_arn`](/configuration/rack-parameters/aws/ecr_additional_policy_arn) instead.

## Default Value

The default value for `ecr_full_access` is `false`.

## Use Cases

- **Restore pre-3.24.6 behavior**: Racks with CI pipelines that push to ECR repos outside the rack naming convention (`${rack_name}/*`).
- **Shared ECR repositories**: Racks where multiple teams push to shared ECR repos that don't follow the rack prefix pattern.
- **Quick recovery**: Immediate fix for 3.24.6 upgrade breakage without auditing which specific repos are needed.

## Setting Parameters

```bash
$ convox rack params set ecr_full_access=true -r rackName
Setting parameters... OK
```

## Disabling

```bash
$ convox rack params set ecr_full_access=false -r rackName
Setting parameters... OK
```

Disabling cleanly detaches the managed policy from the API role. The scoped inline policy (always present) continues to grant access to rack-prefixed repositories.

## Additional Information

- The managed policy is attached via a conditional `aws_iam_role_policy_attachment` resource gated by `count`. Disabling cleanly detaches it with no orphaned resources.
- This parameter can be combined with `ecr_additional_policy_arn`. Both policies are additive.
- Downgrade safety: the `reconcileVarsWithModule` mechanism strips the variable on downgrade to a version without it. Terraform cleanly detaches the policy via `DetachRolePolicy`.

## See Also

- [ecr_additional_policy_arn](/configuration/rack-parameters/aws/ecr_additional_policy_arn) for attaching a custom-scoped IAM policy instead of full access
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for ECR pull-through cache
- [ecr_scan_on_push_enable](/configuration/rack-parameters/aws/ecr_scan_on_push_enable) for image vulnerability scanning
