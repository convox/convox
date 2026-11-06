---
title: "ack_by Derivation"
description: "How the actor field on budget audit events changed in 3.24.6 to carry the authenticated user's email instead of rack-password, and how receivers should adapt."
slug: ack-by-derivation
url: /migration/ack-by-derivation
---
# ack_by Derivation (Migration Note)

> This page is for developers building webhook receivers or SIEM integrations
> that ingest Convox budget audit events. It describes the behavioral contract
> of the `actor` field, not the rack internals.

In 3.24.5 and earlier, the `actor` field on rack-driven budget events was
always the fixed token `"rack-password"` when the action originated in the
Convox Console. Operators who needed to know "who clicked the reset button"
could not get that signal from the rack's audit log alone.

Starting in 3.24.6, operator-triggered actions carry the authenticated user's
identity instead. When a Console action mutates a budget, the user's email is
passed to the rack as `ack_by`, normalized, and emitted as the `actor` field
on the resulting audit event.

This page documents the migration shape for receivers that ingest these
events.

## What changed

| Source | Pre-3.24.6 actor | Post-3.24.6 actor |
|--------|------------------|-------------------|
| Console reset button | `rack-password` | `alice@example.com` (authenticated user) |
| Console dismiss-recovery | `rack-password` | `alice@example.com` |
| Console cap raise | `rack-password` | `alice@example.com` |
| `convox budget reset` CLI | empty / system | unchanged (derives from CLI auth) |
| Automatic, rack-generated events | `system` | `system` (unchanged) |

Events the rack generates on its own (for example when a budget arms, fires,
expires, or is automatically restored) continue to set `actor = "system"`.
Only operator-triggered actions carry a per-user identity.

## Normalization of the actor value

The rack normalizes the incoming `ack_by` value before stamping it on the
event, so receivers can trust the `actor` field without re-sanitizing it
client-side. The behavioral guarantees are:

- Invisible and non-printable characters are removed. This includes control
  characters, zero-width characters, byte-order marks, line and paragraph
  separators, and bidirectional-text override characters that could be used to
  spoof how the value displays in an audit log.
- The value is capped at 256 characters; anything longer is truncated.
- A value that is empty or whitespace-only after normalization becomes the
  literal string `unknown`.

In practice an ordinary email address passes through unchanged. The
normalization only affects values that contain hidden or malformed characters.

## Receiver migration

Receivers that fail-closed when the actor field does not match a known
allowlist must update for the new cardinality. Either:

- Move from a fixed-token allowlist to "any authenticated email" parsing.
- Accept both the legacy `rack-password` (for racks not yet upgraded to
  3.24.6) and the new email shape during the transition window.
- Treat the actor field as opaque and key on the event type for routing.

Webhook receivers that derive identity from a different signal (e.g. the
HMAC verification of the `Convox-Signature` header proving rack identity)
can continue ignoring the actor field.

## Backward compatibility

Pre-3.24.6 racks that have not been upgraded continue to emit
`actor = "rack-password"`. Cross-rack receivers (Slack/Discord webhooks,
external SIEMs) handling events from a mix of rack versions should parse
both shapes: the literal `"rack-password"` from older racks, and an email
or `"system"` value from 3.24.6+ racks.

## See Also

- [Webhook Signing](/console/webhook-signing): header semantics
- [Budget Caps](/management/budget-caps): events that carry the actor field
- [Rack Roles](/console/rack-roles): who can trigger which mutations
