---
title: "Autoscale Triggers Override"
slug: autoscale-triggers
url: /console/autoscale-triggers
---
# Autoscale Triggers Override

The Convox Console exposes a Service-level autoscale configuration surface
that lets you enable, edit, and disable a Service's autoscaler entirely
through the UI — no `convox.yml` edits required. Console-driven autoscale
state persists across deploys via an annotation pair on the Service
Deployment, mirroring the existing scale-override pattern.

This page documents the three states the surface walks through, the four
trigger types you can configure, and what happens to a manifest-declared
autoscaler when you enable an override.

## States

Each Service has one of three autoscale states surfaced in its Scaling tab:

| State | Badge | Action |
|---|---|---|
| Not configured | `Autoscale: Not configured` | `Enable triggers` |
| From `convox.yml` | `Autoscale: From convox.yml` | `Override triggers` |
| Override active | `Autoscale: Override active` | `Disable override` |

**Not configured** — the manifest has no autoscale block and the user has
not enabled an override. The Service runs at a fixed replica count.

**From convox.yml** — the manifest declares `scale.autoscale` or
`scale.targets` (or `scale.count: N-M`), and the Rack has materialized
the matching autoscaler (KEDA `ScaledObject` or native HPA). The Console
shows the configured thresholds and offers an `Override triggers` action.

**Override active** — the user has enabled a Console-driven autoscaler.
The deploy controller respects the override across deploys; the
manifest's autoscale config remains in the YAML but does not materialize
until the override is disabled.

## Trigger Types

The Console exposes four trigger types:

| Type | Backing CRD | Requires |
|---|---|---|
| CPU utilization | Kubernetes HPA | nothing — works on every Rack |
| Memory utilization | Kubernetes HPA | nothing — works on every Rack |
| GPU utilization | KEDA ScaledObject | `keda_enable=true` and `scale.gpu.count >= 1` on the Service |
| Queue depth | KEDA ScaledObject | `keda_enable=true` |

The hybrid CRD dispatch keeps the autoscaler running with the minimum
required machinery. A Service whose triggers are all CPU and/or memory
uses a native HPA, even on Racks without KEDA installed. A Service that
includes GPU or queue-depth triggers uses a KEDA `ScaledObject`; the
Console preflight rejects the request when `keda_enable=false`.

GPU triggers additionally require the Service to declare
`scale.gpu.count >= 1` in `convox.yml`. The KEDA Prometheus trigger
filters by App + Service labels, and without a GPU reservation the
Prometheus query returns nothing forever — autoscale would silently no-op.
The preflight rejects the request with a friendly error so users know
what to fix.

## Enable / Override / Disable

### Enable Triggers

Click `Enable triggers` on a Service with no current autoscale
configuration. The dialog asks for:

- Min replicas (default: current replica count)
- Max replicas (default: `max(count * 3, 5)`)
- Trigger checkboxes. Check the trigger types you want; each row carries
  an inline threshold input pre-filled with a sensible default (CPU 70%,
  Memory 80%, GPU utilization 75%, Queue depth 100).

At least one trigger must be checked to save. The action writes the
`convox.com/triggers-override-active=true` annotation on the Service
Deployment plus a `convox.com/triggers-override-crd=hpa|keda` annotation
recording which CRD was created.

### Override Triggers

Click `Override triggers` on a Service that already has a manifest
autoscale block. The dialog pre-populates with the current manifest
thresholds; unchecking a row drops that trigger from the override save.
This is the typical "turn the existing autoscaler over to the Console
for ongoing edits" path.

### Pencil Edits

When override is active, each threshold cell shows a pencil icon.
Click it to edit the threshold inline; check the value to save. The
edit updates the active CRD directly and emits an
`app:triggers-override:threshold-set` audit event — no full re-enable
required for incremental tuning.

### Disable Override

Click `Disable override` to remove the Console-driven autoscaler. The
matching CRD is deleted, both annotations clear, and on the next deploy
the manifest's autoscale config (if any) re-materializes. If the
manifest has no autoscale block, the Service falls back to fixed
replicas at `scale.count`.

## CLI Parity

The same operations are available from the CLI for users who prefer it:

```bash
$ convox services triggers enable web --min 1 --max 5 --cpu 70 -a my-app
$ convox services triggers disable web -a my-app
$ convox services triggers threshold-set web --type cpu --threshold 80 -a my-app
```

The `--type` argument accepts `cpu`, `memory`, `gpu`, or `queue`. The CLI
gates the same Rack-version preflight (3.24.6+) and surfaces the same
KEDA / GPU-reservation preflight errors as the Console.

## What Persists, What Doesn't

Persists across deploys:

- The Console-driven autoscaler (HPA or KEDA `ScaledObject`).
- Both override annotations on the Deployment.
- The replica count is governed by the autoscaler — same as
  manifest-declared autoscale.

Resets on disable:

- The matching CRD is deleted.
- Both annotations are cleared.
- The next deploy re-materializes the manifest's autoscale config (or
  removes the autoscaler entirely if the manifest has none).

## Limits

The Console-driven override is intentionally a focused surface. Advanced
autoscale knobs stay in `convox.yml`:

- Cooldown / polling interval / stabilization window configuration.
- Custom KEDA trigger types (`scale.keda.triggers` block with raw KEDA
  metadata — e.g. SQS, Pub/Sub, custom Prometheus queries).
- Vertical Pod Autoscaler — independent of triggers and out of scope
  for this surface.

For those, edit `convox.yml` and redeploy. The Console reflects the
materialized state but does not edit cooldown / polling / custom
triggers.

**Heads-up on `scale.keda.triggers` users:** Enabling a Console-driven
override takes ownership of the Service's KEDA ScaledObject `spec.triggers`
array. The four override-managed triggers (CPU, Memory, GPU utilization,
Queue depth) replace whatever was in `spec.triggers` previously, including
any `scale.keda.triggers` passthrough triggers (SQS, Pub/Sub, custom
Prometheus queries). Cooldown, polling interval, advanced KEDA behaviors,
and the manifest's `scale.keda.triggers` passthrough come back on the
next deploy after you click Disable override.

## Authorization

The triggers-override actions are Read+Write RBAC-gated, identical to
deploy / env edit / scale override. Rack-side controllers enforce
`CanWrite`; the Console resolver enforces `requireAppWrite`. Each
action emits an audit event (`app:triggers-override:toggled` or
`app:triggers-override:threshold-set`) carrying the authenticated
user's identity for the audit stream.

## Rack Version Requirements

The override surface and CLI subcommands require Rack version 3.24.6 or
later. Earlier Racks do not know about the new annotations or endpoints
and will return 404 if a 3.24.6 CLI / Console targets them. The Console
checks the Rack version at request time and surfaces a clean
upgrade-required error rather than letting the call fall through.

## Operational Considerations

- **HPA-backed autoscale requires `min >= 1`.** The Kubernetes
  `HPAScaleToZero` feature gate is alpha and is typically not enabled on
  managed clusters (EKS does not enable it). The Console rejects an HPA
  override request with `min=0` and points you at the KEDA-eligible
  trigger types — KEDA's `ScaledObject` supports scale-to-zero natively.
- **Concurrent Enable + in-flight deploy race.** If a `convox deploy`
  is rendering templates at the moment you enable an override, the
  deploy's annotation read can land before your annotation write. That
  particular deploy will emit the manifest autoscaler; the next deploy
  honors the override and the Rack converges. The Console-driven CRD
  remains in place across this race window — no replica thrash, only a
  brief moment where two deploys could disagree about who owns the
  autoscaler.
- **Two operators enabling concurrently.** If two users click Enable
  at the same moment (or the same user double-submits), both Enable
  calls run against the same Deployment. The Kubernetes API server
  rejects whichever CRD-create or CRD-update arrives second (either
  `AlreadyExists` if both writes try Create, or `Conflict` on the
  later Update against a stale ResourceVersion). The losing user sees
  the Rack's API error surfaced in the Console as an error toast and
  can retry; the next attempt observes the now-active override and
  performs an in-place update. The annotation pair converges on the
  last successful write. Operationally safe; no data loss, no
  duplicate CRD risk.
- **CRD-state observation.** KEDA's pollingInterval (default 30s) means
  threshold changes via the pencil affordance take up to one polling
  cycle to reflect in actual scaling decisions. Native HPA's loop is
  ~15s. The Console UI updates the threshold immediately, but the
  cluster's scaling decision honors the new threshold on the next
  reconcile.

## See Also

- [KEDA Autoscaling](/configuration/scaling/keda)
- [Service Detail](/console/service-detail)
- [Scaling](/configuration/scaling)
