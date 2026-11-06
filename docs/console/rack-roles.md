---
title: "Rack Roles"
description: "Rack Roles control who can perform sensitive Rack-level operations in the Console, separate from the per-App RBAC roles that govern who can deploy an App."
slug: rack-roles
url: /console/rack-roles
---
# Rack Roles

Rack Roles control who can perform sensitive Rack-level operations in the Console. This is separate from the per-App RBAC roles documented at [Console RBAC](/management/rbac). Rack Roles answer "who can manage this Rack" while App roles answer "who can deploy this App."

## Organization Administrator vs. Member

Each Convox organization has two levels of access at the Rack tier:

| Role | Capabilities |
|------|-------------|
| **Organization Administrator** | Install Racks, transfer ownership, manage org-level integrations, reveal sensitive Rack-level secrets (e.g. `webhook_signing_key` plaintext). Full access to all Rack operations. |
| **Organization Member** | View Rack parameters with sensitive values masked. Deploy and manage Apps according to their per-App RBAC role. Cannot reveal Rack-level secrets. |

Non-administrators who attempt to reveal a sensitive value receive an access denied error. The reveal control is hidden in the Console UI for non-administrators, and the server enforces the restriction independently.

## What the Administrator Gate Covers

The organization administrator gate currently covers:

- **Revealing `webhook_signing_key` plaintext** in Console > Rack > Settings > Webhook Signing. Members see a masked value only.
- Future Rack-level secret reveals will use the same gate.

App-level operations (deploy, scale, env, Releases) continue to use per-App RBAC roles (`Administrator`, `Operator`, `Developer`, `ReadOnly`, plus V2 variants) documented at [Console RBAC](/management/rbac). The organization administrator gate is in addition to those roles, not a replacement.

## Audit

Every Rack-tier action emits an audit event. As of 3.24.6, the `actor` field on Rack events contains the email of the user who performed the action. Webhook receivers that previously relied on a fixed actor string should update to handle per-user actor values.

## See Also

- [Console RBAC](/management/rbac)
- [Webhook Signing](/console/webhook-signing)
- [ack_by Derivation](/migration/ack-by-derivation)
