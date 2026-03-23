---
title: "Password Security"
slug: password-security
url: /management/password-security
---

# Password Security

Convox Console enforces password complexity requirements to help organizations meet security and compliance standards, including SOC 2 controls.

## Password Requirements

All Convox Console passwords must meet the following criteria:

- **Minimum length**: 12 characters
- **Complexity**: Must contain at least one uppercase letter, one lowercase letter, one number, and one special character

These requirements are enforced across all password flows, including:

- Account creation
- Password reset ("forgot password" flow)
- Password change (from account settings)

Passwords that do not meet these requirements will be rejected with a validation error.

## Third-Party Authentication

As an alternative to password-based authentication, Convox Console supports signing in with **GitHub**. Using GitHub authentication allows your organization to leverage GitHub's own password and authentication policies, including two-factor authentication (2FA).

If your organization requires stricter password policies than Convox enforces natively, using GitHub SSO can help satisfy those requirements indirectly.

## Compliance Considerations

For organizations undergoing compliance audits (e.g., SOC 2), the enforced password requirements align with common control frameworks:

- **Length**: Minimum of 12 characters satisfies most baseline length requirements.
- **Complexity**: Requiring an uppercase letter, lowercase letter, number, and special character addresses common complexity controls.

If your auditor requires documentation of Convox's password policy, you can reference this page as evidence of the enforced requirements.

For additional access control features, see [Console RBAC](/management/rbac) for role-based permission management and [Deploy Keys](/management/deploy-keys) for securing CI/CD access.
