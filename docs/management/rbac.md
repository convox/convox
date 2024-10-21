---
title: "Console RBAC"
draft: false
slug: RBAC
url: /management/rbac
---

# Role-Based Access Control (RBAC)

The RBAC feature allows you to define granular access control by assigning specific roles and permission policies to users in your organization. 

Existing users retain their **Legacy Role** by default, ensuring that no active changes are made upon this update. Organizations can create, test, and assign new roles without affecting current user access.

> **Note**: While legacy roles did not affect CLI access, the new RBAC roles will apply to both the Console and the CLI, ensuring unified permission management across interfaces.

> **Important**: It is highly recommended to restrict write access to the **Roles** resource. If a user has write access to roles, they could potentially modify or escalate their own permissions, undermining the security of the RBAC configuration. Follow the principle of least privilege to mitigate this risk.

## Creating Roles

Roles can be created or cloned using the **Roles** tab found on the **Users** page of the Console. To create a new role:

1. Go to the **Users** page.
2. Select the **Roles** tab.
3. Click **Create Role** to define a new role or **Clone** to create a role based on an existing template.

## Assigning Roles to Users

To assign a custom role to a user:

1. Navigate to the **Active Users** tab.
2. Select the custom role you've created from the dropdown.
3. Assign it to the selected user.

## Pre-Created Roles

To make role assignment easier, Convox provides a set of **pre-created roles**. These roles come with pre-configured permissions, mirroring the existing legacy Console Roles, that are managed by Convox, ensuring they cover common use cases and best practices for platform access. These roles are cumulative, with each level adding more permissions than the last.

### Administrator

The **Administrator** role provides full access across all resources and permissions within the platform.

An Administrator can:
- **Write All Resources**: Full control over all platform resources, including:
  - Billing
  - Users
  - Audit Logs

### OperatorV2

The **OperatorV2** role builds on the **DeveloperV2** role by adding permissions to manage infrastructure and organizational workflows. Operators can perform day-to-day management of key systems and settings without having full control over sensitive areas like billing or user management.

An OperatorV2 can:
- **Write Applications**: Full control over application deployment and management.
- **Write Racks**: Full control over infrastructure and rack configurations.
- **Write Integrations**: Full control over external integrations.
- **Write Workflows**: Full control over workflows.
- **Read Audit Logs**: View-only access to audit logs for monitoring and compliance.
- **Read Dashboard**: View-only access to the organization dashboard.
- **Read Jobs**: View-only access to job statuses and logs.

### DeveloperV2

The **DeveloperV2** role is focused on application development and deployment. This role grants control over applications while providing read-only visibility into certain organizational resources.

A DeveloperV2 can:
- **Write Applications**: Full control over application deployment and management.
- **Read Dashboard**: View-only access to the organization dashboard.
- **Read Jobs**: View-only access to job statuses and logs.
- **Read Racks**: View-only access to rack details.

## Defining Role Permissions

Each role can have multiple permission policies. Every policy consists of:
- **Resource Type**: The type of resource the permission applies to.
- **Resource Name**: The specific name of the resource (when applicable).
- **Action**: Defines whether the permission grants `Read` or `Write` access.

> **Note**: Not all resource types require a resource name. For example, the **Billing** page only provides `Read` or `Write` access without the need to specify a resource name.

### Resource Types

Here are the available resource types that can be selected when defining permissions:

- All Resources
- Application
- Audit Logs
- Billing
- Integrations
- Jobs
- Kubectl Access
- Organization Settings
- Racks
- Roles
- Support
- Users
- Workflows

### Relation Between Rack and App Permissions

It’s important to note that **Rack** permissions and **App** permissions are related. A user must have access to the **Rack** to make use of **App** permissions. If a user is granted app-level permissions but lacks permissions for the associated rack, the app permissions will not grant access. Rack permissions act as a foundational requirement for app-related actions.

### Resource Name Options

For resources requiring a name, you can specify the target resource using one of the following options:

- **Name (string)**: Manually input the name of the app or rack.
- **List**: Select from a dropdown list of available resources.
- **Regex**: Apply a regular expression filter to match resource names.
- **Allow-All**: Grants access to all resource names under the selected resource type.
- **Deny-All**: Denies access to all resource names under the selected resource type.

### Actions

Each permission can either grant `Read` or `Write` access. If access is not explicitly defined, it defaults to denial, following a zero-trust approach.

### Zero Trust Model

RBAC operates under a zero-trust model. This means if no permission is granted, the user has no access to the resource. This ensures that only explicitly allowed actions and resource accesses are permitted.

### All Resources Type

Permissions set for the **All Resources** type are evaluated last. This allows you to use **All Resources** as a base template (e.g., read access) and then override it with more specific policies like `deny-all` for certain resources, making access management easier and more flexible.

## Common Role Patterns

Below are common patterns for role creation using RBAC, addressing specific use cases within an organization.

### 1. Non-Billing Administrator

This role provides administrative-level write access to all resources except for the **Billing** page, where the user is denied access. Additionally, the role has read-only access to the **Users** and **Roles** resources, which is helpful for auditing and oversight without the ability to modify roles.

![Non-Billing Administrator](/images/documentation/management/rbac/example1.png)

### 2. Engineer with Limited Write Access

This role gives an engineer write access to manage deployments and jobs across multiple applications but does not allow modifications to **Racks** or **Organization Settings**. This is useful for teams that need to deploy code and manage app-specific jobs but should not alter infrastructure-level settings.

![Engineer with Limited Write Access](/images/documentation/management/rbac/example2.png)

### 3. Read-Only Auditor for Compliance

This role is designed for compliance or auditing purposes. The user is granted read access to all resources, ensuring they can review configurations, logs, and settings but cannot make any modifications. This is ideal for security audits or compliance checks where full visibility is required without the ability to change anything.

![Read-Only Auditor](/images/documentation/management/rbac/example3.png)

These examples showcase the flexibility of RBAC in managing user access based on common organizational roles and responsibilities.

## Deploy Keys and RBAC

With the RBAC update, you can assign custom roles to [Deploy Keys](/management/deploy-keys), allowing deploy keys to have more flexible permissions. Deploy keys are API keys designed for use in CI environments or other remote systems.

> **Note**: Deploy keys will only utilize permissions related to **Racks** and **Applications**. Permissions such as **Users** or **Billing** will not apply to deploy keys.

By assigning a custom role to a deploy key, you can extend its default capabilities to include additional commands beyond the predefined set. This lets you configure deploy keys with specific permissions tailored to your organizational needs.

For more information on how to create and use deploy keys, visit the [Deploy Keys documentation](/management/deploy-keys).

## Summary

With RBAC, you can now:
- Create custom roles with fine-grained permissions.
- Assign roles to users from the **Active Users** tab.
- Define permissions for different resource types, including applications, billing, and Kubernetes access.
- Leverage the zero-trust model to ensure that only allowed actions are permitted.
- Extend Deploy Key functionality by assigning custom roles for more flexible use cases.

By setting up permissions correctly, your organization can achieve tighter security controls and more flexible user access management.
