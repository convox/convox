---
title: "ack_by Derivation"
slug: ack-by-derivation
url: /migration/ack-by-derivation
---
# ack_by Derivation (Migration Note)

In 3.24.5 and earlier, the `actor` field on rack-driven budget events was
derived from a fixed token (`"rack-password"`) when the action originated in
the Convox Console. Operators who needed to know "who clicked the reset
button" could not get that signal from the rack's audit log alone.

3.24.6 closes that gap by:

1. Plumbing the authenticated user's email through the Console-to-rack call
   chain as the `ack_by` parameter on every mutation route.
2. Sanitizing the value at the rack edge (control-character strip, zero-width
   strip, max-length truncation, whitespace fallback to `"unknown"`) so a
   pathological client cannot stamp a misleading actor.
3. Emitting the sanitized value as the `actor` field on every audit event.

This page documents the migration shape for receivers that ingest these
events.

## What changed

| Source | Pre-3.24.6 actor | Post-3.24.6 actor |
|--------|------------------|-------------------|
| Console reset button | `rack-password` | `alice@example.com` (authenticated user) |
| Console dismiss-recovery | `rack-password` | `alice@example.com` |
| Console cap raise | `rack-password` | `alice@example.com` |
| `convox budget reset` CLI | empty / system | unchanged — derives from CLI auth |
| Rack-internal accumulator tick | `system` | `system` (unchanged) |

Internally-generated events (accumulator-driven `:armed`, `:fired`, `:expired`,
`:flap-suppressed`, automatic `:restored`) continue to set `actor = "system"`.
Only operator-triggered actions carry the per-user identity.

## Sanitization rules

The rack passes the raw `ack_by` through `sanitizeAckBy` before stamping the
event:

- C0 controls (`< 0x20`) and DEL (`0x7F`) — stripped.
- C1 controls (`0x80–0x9F`) — stripped (legacy terminal escapes).
- BiDi overrides (`U+202A`–`U+202E`, `U+2066`–`U+2069`) — stripped (display
  spoofing).
- Line/paragraph separators (`U+2028`, `U+2029`) — stripped.
- Zero-width and BOM (`U+200B`, `U+200C`, `U+200D`, `U+200E`, `U+200F`,
  `U+FEFF`) — stripped (invisible-character spoofing of audit-log values).
- Truncation to 256 characters.
- Whitespace-only collapses to `"unknown"`.

The strip-set is enforced at the rack so receivers can trust the actor field
without re-sanitizing client-side.

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
external SIEMs) handling events from a mix of rack versions should plan to
parse both shapes through the deprecation cycle. 3.25.0 will remove the
fallback path; from 3.25.0 onward the `actor` field is always either an
email or `system`.

## See Also

- [Webhook Signing](/console/webhook-signing) — header semantics
- [Budget Caps](/management/budget-caps) — events that carry the actor field
- [Rack Roles](/console/rack-roles) — who can trigger which mutations
