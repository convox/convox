---
title: "eks_log_types"
slug: eks_log_types
url: /configuration/rack-parameters/aws/eks_log_types
---

# eks_log_types

## Description
Comma-separated list of EKS control plane log types to enable. When set, the EKS cluster ships the specified log categories to CloudWatch Logs at `/aws/eks/<cluster-name>/cluster`.

EKS control plane logging is required by many compliance frameworks (SOC 2, PCI DSS, HIPAA) for audit trail and incident response. Without this parameter, Convox's Terraform applies `enabled_cluster_log_types = []`, which means no control plane logging, and any logging enabled manually through the AWS console is overwritten on the next rack update.

## Default Value
The default value is an empty string (`""`), which means no EKS control plane logging is enabled. This preserves the existing behavior for racks that do not need audit logging.

## Valid Log Types
The following log types are supported by EKS (case-sensitive):
- `api`: Kubernetes API server logs (all API requests)
- `audit`: Kubernetes audit logs (who did what, when; the compliance-critical one)
- `authenticator`: AWS IAM Authenticator logs (authentication events)
- `controllerManager`: Kubernetes controller manager logs
- `scheduler`: Kubernetes scheduler logs

## Use Cases
- **SOC 2 compliance**: Enable `audit` (minimum) to satisfy audit trail requirements. Most auditors expect `api,audit,authenticator` for full coverage.
- **Incident response**: Enable `api,audit` to trace what API calls were made during a security incident, by whom, and from which source IP.
- **Debugging cluster behavior**: Enable `controllerManager,scheduler` to troubleshoot pod scheduling failures or controller reconciliation issues.
- **Full coverage**: Enable all five types for maximum visibility: `api,audit,authenticator,controllerManager,scheduler`.

## Setting Parameters
Enable audit logging (most common compliance requirement):
```bash
$ convox rack params set eks_log_types=audit -r rackName
Setting parameters... OK
```

Enable full SOC 2 coverage:
```bash
$ convox rack params set eks_log_types=api,audit,authenticator -r rackName
Setting parameters... OK
```

Enable all log types:
```bash
$ convox rack params set eks_log_types=api,audit,authenticator,controllerManager,scheduler -r rackName
Setting parameters... OK
```

To disable all logging (revert to default):
```bash
$ convox rack params set eks_log_types= -r rackName
Setting parameters... OK
```

## Additional Information
- Log types are validated by the AWS API at apply time. Invalid values (e.g., `Audit` with a capital A, or `controller_manager` with an underscore) are rejected with an `InvalidParameterException`.
- CloudWatch Logs pricing applies: $0.50/GB ingested, $0.03/GB/month stored. The `api` log type on a busy cluster can generate significant volume, so consider enabling only `audit` if cost is a concern.
- **CloudWatch log group lifecycle**: When logging is enabled, AWS automatically creates a log group at `/aws/eks/<cluster-name>/cluster`. When logging is disabled (parameter cleared or rack downgraded), EKS stops writing to the log group but AWS does **not** delete it. Historical logs and the log group persist until manually deleted. This is standard AWS behavior, not a Terraform artifact.
- Downgrade safety: removing this parameter (or downgrading to a rack version that does not support it) disables logging. The EKS cluster is updated in-place; no destructive changes occur.
- This parameter prevents the common issue where a user enables EKS audit logging manually through the AWS console, and Convox's next Terraform apply silently disables it because the `aws_eks_cluster` resource had no `enabled_cluster_log_types` attribute.

## Related Parameters
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days): Controls Nginx access log retention in CloudWatch (application-level logging, not cluster-level).
- [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable): Controls the Fluentd log collector for application logs.
- [syslog](/configuration/rack-parameters/aws/syslog): Forwards application logs to an external syslog endpoint.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
