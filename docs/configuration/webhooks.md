---
title: "Webhooks"
slug: webhooks
url: /configuration/webhooks
---
# Webhooks

Convox racks emit webhooks for app-level events (deploys, crashes, budget cap
fires, auto-shutdown lifecycle). Receivers — Slack, Discord, PagerDuty, custom
endpoints — subscribe to the relevant events and act on them.

This page describes the webhook configuration shape, the event catalog, and
the delivery semantics. For signature verification details see
[Webhook Signing](/console/webhook-signing).

## Configuring receivers

Webhook URLs are configured per-app in the Console under **App > Settings >
Webhooks**, or via the `convox.yml` `services.<svc>.atCapWebhookUrl` field
for budget-cap events scoped to a specific service.

Every webhook is delivered as an HTTP POST with `Content-Type:
application/json`, signed via the rack's `webhook_signing_key` (see
[Webhook Signing](/console/webhook-signing)).

## Event catalog

Each event has a stable string `event` field. Receivers should switch on this
field for routing.

### App lifecycle

- `app:created`
- `app:deleted`
- `app:locked`, `app:unlocked`
- `app:release:promoted`
- `app:release:rolled-back`

### Build / deploy

- `app:build:started`, `app:build:succeeded`, `app:build:failed`
- `app:deploy:started`, `app:deploy:succeeded`, `app:deploy:failed`

### Service health

- `app:service:scaled`
- `app:service:crashlooping`
- `app:service:restored`

### Budget caps

- `app:budget:threshold` — `alertThresholdPercent` crossed
- `app:budget:cap` — `monthlyCapUsd` crossed; breaker may have tripped
  depending on `atCapAction`

### Auto-shutdown lifecycle (9 events + 2 audit-only)

Per the auto-shutdown specification:

- `app:budget:auto-shutdown:armed`
- `app:budget:auto-shutdown:fired`
- `app:budget:auto-shutdown:cancelled`
- `app:budget:auto-shutdown:restored`
- `app:budget:auto-shutdown:expired`
- `app:budget:auto-shutdown:flap-suppressed`
- `app:budget:auto-shutdown:noop`
- `app:budget:auto-shutdown:failed`
- `app:budget:auto-shutdown:simulated`

Audit-only (not part of the 9 lifecycle events; emitted by Console-driven
operator actions):

- `app:budget:auto-shutdown:dismissed` — 3.24.6+; emitted when the recovery
  banner is dismissed.
- `app:budget:auto-shutdown:breaker-cleared` — 3.24.6+; emitted when a
  cap-raise clears the breaker mid-countdown after `:fired`.

### Signing <a id="signing"></a>

See [Webhook Signing](/console/webhook-signing) for the full HMAC-SHA256
signing protocol, the `Convox-Signature` and `Convox-Signature-Timestamp`
headers, and an example verification routine.

## Payload shape

Every event payload is a JSON object with at least:

```json
{
  "event": "app:budget:auto-shutdown:fired",
  "app": "myapp",
  "rack": "rack1",
  "actor": "alice@example.com",
  "timestamp": "2026-04-27T10:30:00Z",
  "tick_id": "tick-a1b2c3d4"
}
```

Event-specific fields appear alongside the common fields. Receivers should
ignore unknown fields to remain forward-compatible. The `actor` field is the
authenticated identity that triggered the action — for rack-internal events
(accumulator ticks) the value is `"system"`. See [ack_by Derivation](/migration/ack-by-derivation)
for the migration story around the actor field.

## Delivery semantics

Webhooks are best-effort, fire-and-forget, and not retried. Receivers must
handle:

- **Out-of-order arrival** — order events by the `timestamp` field, not by
  receipt order. Two events fired in the same tick (`:set` followed by
  `:breaker-cleared`) are dispatched async and may arrive in either order.
- **Duplicates** — the rack lock prevents in-process duplication, but a
  receiver behind a load balancer or proxy may see the same event twice on
  retry. Idempotency by `tick_id` + `event` is recommended.
- **Single-shot** — the rack does not retry on receiver-side 5xx. Persistent
  storage of the audit stream lives in the Console's Events tab; the webhook
  feed is the operator notification channel, not a transactional queue.
- **Source of truth** — when the Events tab persistence has a gap, the
  Slack/Discord webhook receiver is the authoritative source for the event
  log. Persist webhook payloads if you need replay.

## Filtering and routing

Some receivers (Slack, Discord) prefer not to be paged on routine events
(`:noop`, `:simulated`). Filter at the receiver:

- `:noop` — the auto-shutdown reconciler ran and decided no action was
  needed. High-volume; usually filtered.
- `:simulated` — `convox budget simulate-shutdown --app myapp` ran a dry-run.
  Audit only; usually filtered.
- `:flap-suppressed` — a cap-fire was suppressed because the app recently
  recovered. Useful signal for tuning `monthlyCapUsd` but not actionable.

PagerDuty and on-call channels typically subscribe to `:fired`, `:failed`,
and `app:service:crashlooping`.

## See Also

- [Webhook Signing](/console/webhook-signing) — signature verification
- [Budget Caps](/management/budget-caps) — events that fire on cap crossings
- [ack_by Derivation](/migration/ack-by-derivation) — actor field migration
