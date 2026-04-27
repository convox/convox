---
title: "Rack Roles"
slug: rack-roles
url: /console/rack-roles
---
# Rack Roles

Console organizes racks into ownership tiers that control who can install,
upgrade, set parameters on, or delete a rack. Roles are independent of the
finer-grained app-level RBAC that controls per-app permissions; they answer
"who is allowed to operate this rack" rather than "who is allowed to deploy
this app".

This page documents the rack-tier roles that ship with 3.24.6. For app-level
RBAC see [Console RBAC](/management/rbac).

## Roles

| Role | Capabilities |
|------|--------------|
| **Owner** | Full control. Can install, uninstall, upgrade, change rack params, manage rack-level integrations, transfer ownership, view audit logs. The user who installs the rack via `convox rack install` is the initial Owner. |
| **Admin** | Same as Owner except cannot transfer ownership or uninstall. Used for trusted operators who should not be able to delete the rack. |
| **Operator** | Can change non-destructive rack params (e.g. `idle_timeout`, `fluentd_memory`), trigger `convox rack update` to upgrade to a newer rack version, view audit logs. Cannot change destructive params (e.g. `cidr`, `availability_zones`, `karpenter_*` core toggles). |
| **Viewer** | Read-only access to rack params, version, processes, logs. No mutation. |

App-level roles (Member, Developer, Deployer, Viewer) are documented at
[Console RBAC](/management/rbac); they apply per-app, not per-rack.

## Role assignment

Owners assign roles via the Console UI under **Rack > Settings > Members**.
Each assignment is per-user, per-rack. A user can hold different rack roles on
different racks.

Service accounts and CI/CD bots are typically assigned the Operator or Viewer
role depending on whether they trigger upgrades or only observe.

## Audit

Every rack-tier role mutation (assignment, change, removal) emits an audit
event tagged with the actor's authenticated identity. As of 3.24.6, the
`actor` field on these events is per-user (the email of the operator who made
the change), not the historical `"rack-password"` constant. Webhook receivers
that previously keyed on a fixed actor string must update for the new
cardinality.

## Defaults and migration

Pre-3.24.6 racks installed before role assignment was available default to
Owner = the installer's email; all other organization members default to
Viewer until an Owner promotes them. Existing rack-password-only access
remains supported through 3.25.0 for backward compatibility, then is
deprecated.

## See Also

- [Console RBAC](/management/rbac) — app-level role-based access control
- [Webhook Signing](/console/webhook-signing) — actor field semantics on
  rack-mutation events
- [ack_by Derivation](/migration/ack-by-derivation) — how the actor field is
  derived for rack-driven events
