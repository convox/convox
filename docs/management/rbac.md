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

### Best Practices for Role Creation

A recommended pattern for role creation is to start with **All Resources** set to `Read` access, and then apply more granular `Write` or `Deny` permissions for specific resources. This ensures broad, non-destructive access to resources by default, while allowing finer control over which users have higher privileges or restrictions.

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
