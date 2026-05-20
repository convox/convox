---
title: "Rack Roles"
slug: rack-roles
url: /console/rack-roles
---
# Rack Roles

This page documents the Rack-tier authorization that the Console enforces in
3.24.6 — primarily the **organization administrator** gate that controls
sensitive Rack operations such as revealing the `webhook_signing_key`. Rack-tier
authorization is independent of the per-App RBAC roles documented at
[Console RBAC](/management/rbac); they answer "who is allowed to operate this
Rack" rather than "who is allowed to deploy this App".

## Organization Administrator Gate

Each Convox organization stores a list of administrator user IDs on the
organization record. A user is either:

- **Organization administrator** — listed in the org's administrator set. Can
  install Racks, transfer ownership, manage org-level integrations, and view or
  reveal sensitive Rack-level secrets (e.g. the `webhook_signing_key` plaintext
  via the Console reveal action introduced in 3.24.6).
- **Organization member** — every other authenticated user in the org. Can
  view Rack params with sensitive values masked, deploy Apps per the per-App
  RBAC role assigned to them, and operate the Racks they have access to. Cannot
  reveal sensitive Rack-level secrets.

The distinction is binary at the Rack tier: the Console GraphQL resolver gates
the unmasked-reveal path on org-administrator membership directly, so a member
calling the resolver from a custom client will receive an `access denied` error
rather than the plaintext secret. The Vue layer additionally disables the
reveal control in the UI for non-administrators, but the server-side check is
the authoritative check.

## What the Gate Covers

As of 3.24.6, the org-administrator gate covers:

- Revealing `webhook_signing_key` plaintext in **Console > Rack > Settings >
  Webhook Signing**. Members see the masked sentinel only.
- Future Rack-level secret reveals will use the same gate.

App-level mutations (deploy, scale, env, Releases) continue to use the per-App
RBAC roles (`Administrator`, `Operator`, `Developer`, `ReadOnly`, plus the V2
variants) documented at [Console RBAC](/management/rbac). The org-administrator
gate is in addition to — not a replacement for — those roles.

## Audit

Every Rack-tier action emits an audit event tagged with the actor's
authenticated identity. As of 3.24.6, the `actor` field on Rack-mutation events
is per-user (the email of the operator who made the change), not the historical
`"rack-password"` constant. Webhook receivers that previously keyed on a fixed
actor string must update for the new cardinality.

## See Also

- [Console RBAC](/management/rbac) — App-level role-based access control
- [Webhook Signing](/console/webhook-signing) — the reveal flow gated by the
  org-administrator membership check
- [ack_by Derivation](/migration/ack-by-derivation) — how the actor field is
  derived for Rack-driven events
