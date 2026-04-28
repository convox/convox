---
title: "Webhooks"
slug: webhooks
url: /configuration/webhooks
---
# Webhooks

Convox racks emit webhooks for app-level events (app create, build/release
lifecycle, budget cap fires, auto-shutdown lifecycle). Receivers — Slack,
Discord, PagerDuty, custom endpoints — subscribe to the relevant events and
act on them.

This page describes the webhook configuration shape, the event catalog, and
the delivery semantics. For signature verification details see
[Webhook Signing](/console/webhook-signing).

## Configuring receivers

Webhook URLs are configured per-app in the Console under **App > Settings >
Webhooks**, or via the `convox.yml` top-level `budget.atCapWebhookUrl` field
for budget-cap events. The field lives on the manifest-level `budget:` block
and applies to the whole app — there is no per-service webhook URL.

Every webhook is delivered as an HTTP POST with `Content-Type:
application/json`, signed via the rack's `webhook_signing_key` (see
[Webhook Signing](/console/webhook-signing)).

## Event catalog

Each event has a stable string `action` field. Receivers should switch on this
field for routing. The list below enumerates the events emitted by 3.24.6.
Receivers should ignore unknown actions to remain forward-compatible with
future rack versions.

Webhook events fall into two classes:

- **HTTP-handler events** — emitted synchronously from a request handler in
  response to a customer or operator action. The request's authenticated
  identity propagates to `data.actor` (JWT-derived email for Console3-driven
  mutations, the rack's audit actor for CLI-driven mutations).
- **Accumulator-tick events** — emitted asynchronously by the in-rack budget
  accumulator or render-time advisory paths, with no HTTP request in scope.
  These hardcode `data.actor: "system"`.

The actor class for each event is noted alongside the action below.

### App lifecycle

- `app:create` — emitted after `convox apps create` succeeds (HTTP-handler;
  JWT actor).

### Build / release

- `build:create` — emitted at build start and on build failure (HTTP-handler;
  JWT actor). The `status` field distinguishes success (`"success"`) from
  failure (`"error"` with `data.message` carrying the build error).
- `build:import-image:start`, `build:import-image:done` — emitted at the
  start and end of `convox build import-image` (HTTP-handler; JWT actor
  captured before the async finalize goroutine).
- `release:create` — emitted after a release is created from a successful
  build (HTTP-handler; JWT actor).
- `release:promote` — emitted at promote start (HTTP-handler; JWT actor).
  `status: "start"` on this event distinguishes it from later release
  events.
- `release:autoscale-disabled` — emitted at render time when a service
  requests autoscale on a rack without `keda_enable=true`
  (accumulator-tick / render-time; `actor: "system"`).
- `release:manifest-advisory` — emitted at render time when a service
  configuration is degenerate (e.g., `scale.min: 0` without autoscale)
  (render-time; `actor: "system"`).
- `release:prometheus-default` — emitted at render time when KEDA needs
  Prometheus and no explicit `prometheus_url` is configured
  (render-time; `actor: "system"`).
- `release:imperative-patch-note` — emitted when `convox scale` rewrites
  a KEDA-managed service to patch the ScaledObject instead of the
  Deployment (HTTP-handler; `actor: "system"`).

### Budget cap & cost (3.24.6)

- `app:budget:set` — emitted after `convox budget set` or a Console-driven
  budget config write (HTTP-handler; JWT actor populated from `data.ack_by`).
  Carries previous and new cap, action, threshold, and pricing-adjustment
  values.
- `app:budget:reset` — emitted after `convox budget reset` clears the
  circuit breaker (HTTP-handler; JWT actor).
- `app:budget:clear` — emitted after `convox budget clear` removes the
  budget config (HTTP-handler; JWT actor). Carries the prior-state
  snapshot so an auditor can reconstruct what was destroyed.
- `app:budget:threshold` — `alertThresholdPercent` crossed
  (accumulator-tick; `actor: "system"`).
- `app:budget:cap` — `monthlyCapUsd` crossed; breaker may have tripped
  depending on `atCapAction` (accumulator-tick; `actor: "system"`).
- `app:budget:breaker-cleared` — emitted when a cap-raise clears the deploy
  circuit breaker (both during the armed countdown and post-`:fired`)
  (HTTP-handler; JWT actor populated from `data.ack_by`). NOT a sub-type
  of `auto-shutdown`.

### Auto-shutdown lifecycle (3.24.6)

Auto-shutdown is a sub-family of budget events. Most lifecycle events are
accumulator-tick driven and emit `actor: "system"`. Sub-cases driven by
HTTP handlers (a customer action that aborts an armed countdown) carry the
JWT-derived actor.

- `app:budget:auto-shutdown:armed` — armed countdown begins
  (accumulator-tick; `actor: "system"`).
- `app:budget:auto-shutdown:fired` — countdown elapsed; services scaled
  to zero (accumulator-tick; `actor: "system"`).
- `app:budget:auto-shutdown:cancelled` — emitted when an in-flight armed
  countdown is cancelled before `:fired`. The payload's `cancel_reason`
  field carries one of: `reset-during-armed` (operator ran `convox budget
  reset` during the armed window — HTTP-handler; JWT actor), `cap-raised`
  (the cap was raised mid-armed-window to a value above current spend —
  HTTP-handler when triggered by `convox budget cap raise`; accumulator-tick
  when `convox apps update --manifest` produces both a manifest-SHA change
  AND the new `monthlyCapUsd` exceeds current spend (`cfg.MonthlyCapUsd >
  baseState.CurrentMonthSpendUsd`). If the manifest change does not raise the
  cap above current spend, the same accumulator-tick branch fires
  `config-changed` instead — receivers should not assume `cap-raised` for
  every manifest-SHA change in the armed window. JWT actor flows through
  `cfg.LastCapMutationBy` on both paths), `manual-detected` (an out-of-band manual
  scale-up resolved the breach — accumulator-tick with `actor: "system"`
  on the primary path; HTTP-handler with JWT-derived actor when
  `convox budget reset` is run during the armed window and the customer
  has already manually scaled some services back up, with the operator's
  identity flowing through `data.ack_by`), `config-changed` (the
  budget config was edited mid-armed-window in a way that altered
  eligibility — accumulator-tick; `actor: "system"`).
- `app:budget:auto-shutdown:restored` — services restored from the
  persisted shutdown-state annotation. Tick-driven (`actor: "system"`)
  unless triggered by `convox budget reset` post-`:fired`, in which case
  the JWT-derived actor flows through.
- `app:budget:auto-shutdown:expired` — manual-mode month rollover with
  customer absent (accumulator-tick; `actor: "system"`).
- `app:budget:auto-shutdown:flap-suppressed` — a cap-fire was suppressed
  by the 24-hour cooldown after a recent recovery (accumulator-tick;
  `actor: "system"`).
- `app:budget:auto-shutdown:noop` — reconciler ran and decided no action
  was needed (accumulator-tick; `actor: "system"`).
- `app:budget:auto-shutdown:failed` — shutdown patch retries exhausted
  (accumulator-tick; `actor: "system"`).
- `app:budget:auto-shutdown:simulated` — `convox budget simulate-shutdown
  --app <app>` was run (HTTP-handler; JWT actor).
- `app:budget:auto-shutdown:dismissed` — emitted when the recovery banner
  is dismissed via `convox budget dismiss-recovery` or the Console UI
  (HTTP-handler; JWT actor, falling back to `"system"` if the request
  was unauthenticated).

### Signing <a id="signing"></a>

See [Webhook Signing](/console/webhook-signing) for the full HMAC-SHA256
signing protocol, the single `Convox-Signature` header (format
`t=<unix-ts>,v1=<hex1>[,v1=<hex2>]` — multiple `v1=` segments may
appear during key rotation; receivers verify against any one), and an
example verification routine.

## Payload shape

Every event payload is a JSON object with `action`, `data`, `status`, and
`timestamp` at the top level. Event-specific fields (including the common
`app`, `rack`, `actor`, and where applicable `tick_id`) live under `data`.

The example below is an HTTP-handler event (`:cancelled` reason
`reset-during-armed`, fired when `convox budget reset` is run during the
armed window) — `data.actor` carries the JWT-derived email of the operator
who ran the reset:

```json
{
  "action": "app:budget:auto-shutdown:cancelled",
  "status": "success",
  "timestamp": "2026-04-27T10:30:00Z",
  "data": {
    "app": "myapp",
    "rack": "rack1",
    "actor": "alice@example.com",
    "tick_id": "tick-2026-04-27T10:30:00Z-3a7b4c2d8e9f4a6b8c4d3e2f1a0b9c8d",
    "cancel_reason": "reset-during-armed"
  }
}
```

Event-specific fields appear inside `data` alongside the common fields.
Receivers should ignore unknown fields to remain forward-compatible. The
`data.actor` field is the authenticated identity that triggered the action;
for accumulator-tick events (`:fired`, `:armed`, `:expired`, `:cap`,
`:threshold`, etc.) the value is `"system"` because no HTTP request is in
scope at the trigger point. See the per-event actor classification in the
event catalog above, and [ack_by Derivation](/migration/ack-by-derivation)
for the migration story around the actor field.

## Delivery semantics

Webhooks are best-effort, fire-and-forget, and not retried. Receivers must
handle:

- **Out-of-order arrival** — order events by the `timestamp` field, not by
  receipt order. Two events fired in the same tick (`:set` followed by
  `:breaker-cleared`) are dispatched async and may arrive in either order.
- **Duplicates** — the rack lock prevents in-process duplication, but a
  receiver behind a load balancer or proxy may see the same event twice on
  retry. Idempotency by `data.tick_id` + `action` is recommended.
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

PagerDuty and on-call channels typically subscribe to
`app:budget:auto-shutdown:fired`, `app:budget:auto-shutdown:failed`, and
`app:budget:cap` (when `atCapAction` is `block-new-deploys` or
`auto-shutdown`).

## See Also

- [Webhook Signing](/console/webhook-signing) — signature verification
- [Budget Caps](/management/budget-caps) — events that fire on cap crossings
- [ack_by Derivation](/migration/ack-by-derivation) — actor field migration
