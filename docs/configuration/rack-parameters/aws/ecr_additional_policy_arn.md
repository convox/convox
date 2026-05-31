---
title: "ecr_additional_policy_arn"
slug: ecr_additional_policy_arn
url: /configuration/rack-parameters/aws/ecr_additional_policy_arn
---

# ecr_additional_policy_arn

## Description

The `ecr_additional_policy_arn` parameter attaches a user-provided IAM policy to the Rack API IAM role. This provides fine-grained control over which additional ECR repositories the rack can access beyond the default scoped policy (limited to `${rack_name}/*` and `${rack_name}-*`).

Use this when your CI pipelines push pre-built images to specific ECR repositories outside the rack naming convention and you want to grant access to only those repos rather than using [`ecr_full_access`](/configuration/rack-parameters/aws/ecr_full_access) for blanket access.

## Default Value

The default value for `ecr_additional_policy_arn` is `""` (empty, no additional policy attached).

## Use Cases

- **Scoped cross-repo access**: Grant the Rack API role access to specific ECR repos used by `convox build --cache-from` or `convox builds import-image` without opening all ECR repos.
- **Shared image repositories**: Attach a policy scoped to shared ECR repos used across multiple racks or teams.
- **Compliance-friendly**: Maintain least-privilege access by granting only the specific ECR permissions needed.

## Setting Parameters

Create a custom IAM policy scoped to the repos you need, then attach it:

```bash
$ convox rack params set ecr_additional_policy_arn=arn:aws:iam::123456789012:policy/my-ecr-repos -r rackName
Setting parameters... OK
```

## Clearing

Clear the parameter to detach the custom policy:

```bash
$ convox rack params set ecr_additional_policy_arn= -r rackName
Setting parameters... OK
```

This parameter is clearable. Setting it to an empty value detaches the policy cleanly.

## Example IAM Policy

A minimal policy granting push and pull access to specific ECR repositories:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecr:GetDownloadUrlForLayer",
        "ecr:BatchGetImage",
        "ecr:BatchCheckLayerAvailability",
        "ecr:PutImage",
        "ecr:InitiateLayerUpload",
        "ecr:UploadLayerPart",
        "ecr:CompleteLayerUpload"
      ],
      "Resource": [
        "arn:aws:ecr:us-east-1:123456789012:repository/shared-base-images",
        "arn:aws:ecr:us-east-1:123456789012:repository/ci-cache/*"
      ]
    }
  ]
}
```

## Additional Information

- Clearing the parameter cleanly detaches the custom policy from the API role with no leftover resources.
- This parameter can be combined with `ecr_full_access`. Both policies are additive (though `ecr_full_access` makes this parameter redundant).
- The policy ARN must be a valid IAM policy in the same AWS account. Cross-account policy ARNs are not supported.
- Downgrade safety: on downgrade to a rack version that does not support this parameter, the attached policy is cleanly detached and no manual cleanup is required.

## See Also

- [ecr_full_access](/configuration/rack-parameters/aws/ecr_full_access) for restoring blanket ECR access (pre-3.24.6 behavior)
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for ECR pull-through cache
- [ecr_scan_on_push_enable](/configuration/rack-parameters/aws/ecr_scan_on_push_enable) for image vulnerability scanning
