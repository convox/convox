package k8s

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// shutdownTickIDPrefix is prepended to UUIDv4-style hex tokens to make
// the saved annotation easier to grep for in audit replays.
const shutdownTickIDPrefix = "tick-"

// Nine lifecycle event names. The accumulator loop
// (budget_accumulator.go) fires :armed/:fired/:expired/:flap-suppressed/:noop
// at tick time; this file fires :cancelled/:restored/:failed/:simulated
// from the shutdown/restore code paths. The names are pinned here so a
// grep for app:budget:auto-shutdown: in budget_*.go can find all 9.
//
//	app:budget:auto-shutdown:armed
//	app:budget:auto-shutdown:fired
//	app:budget:auto-shutdown:cancelled
//	app:budget:auto-shutdown:restored
//	app:budget:auto-shutdown:expired
//	app:budget:auto-shutdown:flap-suppressed
//	app:budget:auto-shutdown:failed
//	app:budget:auto-shutdown:simulated
//	app:budget:auto-shutdown:noop
//
// Plus one audit-only event (not a lifecycle event):
// app:budget:auto-shutdown:dismissed fired by the dismiss-recovery path.
const (
	ShutdownEventArmed          = "app:budget:auto-shutdown:armed"
	ShutdownEventFired          = "app:budget:auto-shutdown:fired"
	ShutdownEventCancelled      = "app:budget:auto-shutdown:cancelled"
	ShutdownEventRestored       = "app:budget:auto-shutdown:restored"
	ShutdownEventExpired        = "app:budget:auto-shutdown:expired"
	ShutdownEventFlapSuppressed = "app:budget:auto-shutdown:flap-suppressed"
	ShutdownEventFailed         = "app:budget:auto-shutdown:failed"
	ShutdownEventSimulated      = "app:budget:auto-shutdown:simulated"
	ShutdownEventNoop           = "app:budget:auto-shutdown:noop"
	ShutdownEventDismissed      = "app:budget:auto-shutdown:dismissed"
)

// budgetShutdownPatchRetries is the in-tick retry budget for a single
// PATCH that fails with admission-webhook denial or 409 conflict before
// :failed fires (3 attempts with exponential backoff).
const budgetShutdownPatchRetries = 3

// shutdownEventName returns the qualified event name for a lifecycle
// event (e.g. shutdownEventName("armed") →
// "app:budget:auto-shutdown:armed").
func shutdownEventName(suffix string) string {
	return "app:budget:auto-shutdown:" + suffix
}

// universalEventData returns the universal payload fields every
// lifecycle event carries. Caller fills in event-specific fields after
// calling this helper.
//
// cap_usd is emitted as int (no decimals) so receivers can parse as int;
// spend_usd remains decimal.
func universalEventData(actor, tickID string, dryRun bool, capUsd, spendUsd float64) map[string]string {
	return map[string]string{
		"actor":          actor,
		"tick_id":        tickID,
		"schema_version": strconv.Itoa(structs.BudgetShutdownStateSchemaVersion),
		"dry_run":        strconv.FormatBool(dryRun),
		"cap_usd":        strconv.FormatFloat(capUsd, 'f', 0, 64),
		"spend_usd":      strconv.FormatFloat(spendUsd, 'f', 2, 64),
	}
}

// generateShutdownTickID returns a fresh UUIDv4-like hex token. Used
// once per shutdown event sequence; all events in the sequence share
// the same tick id.
func generateShutdownTickID(now time.Time) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	// Set version (4) and variant bits per RFC 4122 §4.4
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s%s-%s", shutdownTickIDPrefix, now.UTC().Format("2006-01-02T15:04:05Z"), hex.EncodeToString(b[:]))
}

// readBudgetShutdownStateAnnotation parses the
// `convox.com/budget-shutdown-state` annotation. Returns (nil, nil)
// when absent. Corrupt-annotation classes are surfaced as errors so the
// caller can either fire :failed reason=state-corrupt (accumulator-
// driven path) or unconditionally delete (reset-driven path).
func readBudgetShutdownStateAnnotation(ann map[string]string) (*structs.AppBudgetShutdownState, error) {
	raw, ok := ann[structs.BudgetShutdownStateAnnotation]
	if !ok || raw == "" {
		return nil, nil
	}
	var s structs.AppBudgetShutdownState
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, errors.Wrap(err, "budget shutdown state annotation malformed")
	}
	if err := s.ValidateRequiredFields(); err != nil {
		return nil, errors.Wrap(err, "budget shutdown state annotation invalid")
	}
	return &s, nil
}

// readFlapSuppressedUntilAnnotation parses the smaller carry-over
// annotation written by the GC when the main shutdown-state
// annotation expires post-restore. Single ISO-8601 timestamp value.
func readFlapSuppressedUntilAnnotation(ann map[string]string) (*time.Time, error) {
	raw, ok := ann[structs.BudgetFlapSuppressedUntilAnnotation]
	if !ok || raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// readRecoveryBannerDismissedAnnotation parses the dismiss-recovery
// timestamp set by `convox budget dismiss-recovery`.
func readRecoveryBannerDismissedAnnotation(ann map[string]string) (*time.Time, error) {
	raw, ok := ann[structs.BudgetRecoveryBannerDismissedAnnotation]
	if !ok || raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// writeBudgetShutdownStateAnnotation marshals and writes the state
// annotation via Update with the provided resourceVersion as
// optimistic-concurrency precondition. On 409 Conflict the caller is
// expected to re-read with a fresh resourceVersion and retry.
func (p *Provider) writeBudgetShutdownStateAnnotation(ctx context.Context, app string, s *structs.AppBudgetShutdownState, resourceVersion string) error {
	nsName := p.AppNamespace(app)

	// RecoveryBannerDismissedAt is GET-time-only aggregation; nil the
	// field before marshal so the persisted shutdown-state annotation
	// contains only state-machine fields. The dismissed annotation is
	// written separately via writeRecoveryBannerDismissedAnnotation;
	// this nil-clear keeps the two annotations as orthogonal sources of
	// truth. The local copy via cleaned := *s avoids mutating the
	// caller's struct.
	if s != nil {
		cleaned := *s
		cleaned.RecoveryBannerDismissedAt = nil
		s = &cleaned
	}

	raw, err := json.Marshal(s)
	if err != nil {
		return errors.WithStack(err)
	}
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	if resourceVersion != "" && ns.ResourceVersion != resourceVersion {
		return ae.NewConflict(schema.GroupResource{Resource: "namespaces"}, nsName,
			fmt.Errorf("resource version mismatch: expected %s, got %s", resourceVersion, ns.ResourceVersion))
	}
	if ns.Annotations == nil {
		ns.Annotations = map[string]string{}
	}
	ns.Annotations[structs.BudgetShutdownStateAnnotation] = string(raw)
	_, err = p.Cluster.CoreV1().Namespaces().Update(ctx, ns, am.UpdateOptions{})
	return errors.WithStack(err)
}

// deleteBudgetShutdownStateAnnotation removes the state annotation.
// Used by the GC and the unconditional-delete path on AppBudgetReset.
func (p *Provider) deleteBudgetShutdownStateAnnotation(ctx context.Context, app string) error {
	nsName := p.AppNamespace(app)
	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				return nil
			}
			return errors.WithStack(err)
		}
		if ns.Annotations == nil {
			return nil
		}
		if _, ok := ns.Annotations[structs.BudgetShutdownStateAnnotation]; !ok {
			return nil
		}
		delete(ns.Annotations, structs.BudgetShutdownStateAnnotation)
		if _, err := p.Cluster.CoreV1().Namespaces().Update(ctx, ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}
		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to delete budget shutdown state annotation after %d retries", budgetWriteConflictRetries))
}

// writeFlapSuppressedUntilAnnotation persists the cooldown carry-over
// after the main state annotation is GC'd. Single timestamp value.
func (p *Provider) writeFlapSuppressedUntilAnnotation(ctx context.Context, app string, until time.Time) error {
	return p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetFlapSuppressedUntilAnnotation, until.UTC().Format(time.RFC3339))
}

// writeRecoveryBannerDismissedAnnotation marks the recovery banner as
// dismissed.
func (p *Provider) writeRecoveryBannerDismissedAnnotation(ctx context.Context, app string, at time.Time) error {
	return p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetRecoveryBannerDismissedAnnotation, at.UTC().Format(time.RFC3339))
}

// patchNamespaceStringAnnotation upserts a string annotation key on
// the App namespace via Get-Update with conflict retry.
func (p *Provider) patchNamespaceStringAnnotation(ctx context.Context, app, key, value string) error {
	nsName := p.AppNamespace(app)
	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				return errors.WithStack(structs.ErrNotFound("app not found: %s", app))
			}
			return errors.WithStack(err)
		}
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[key] = value
		if _, err := p.Cluster.CoreV1().Namespaces().Update(ctx, ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}
		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to write annotation %s after %d retries", key, budgetWriteConflictRetries))
}

// deleteNamespaceAnnotation removes a single annotation key from the
// App namespace.
func (p *Provider) deleteNamespaceAnnotation(ctx context.Context, app, key string) error {
	nsName := p.AppNamespace(app)
	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				return nil
			}
			return errors.WithStack(err)
		}
		if ns.Annotations == nil {
			return nil
		}
		if _, ok := ns.Annotations[key]; !ok {
			return nil
		}
		delete(ns.Annotations, key)
		if _, err := p.Cluster.CoreV1().Namespaces().Update(ctx, ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}
		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to delete annotation %s after %d retries", key, budgetWriteConflictRetries))
}

// shutdownService runs the per-service shutdown algorithm.
//
// Order:
//  1. State annotation FIRST (atomic-pre-PATCH).
//  2. PodSpec terminationGracePeriodSeconds.
//  3. Deployment.Spec.Replicas=0.
//  4. ScaledObject paused-replicas annotation.
//
// Returns the names of services that successfully shut down and the
// names that failed (for the :failed event payload).
func (p *Provider) shutdownService(ctx context.Context, app, svc string, gracePeriodSeconds int64) error {
	nsName := p.AppNamespace(app)
	dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svc, am.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "get deployment %s/%s", nsName, svc)
	}

	// Idempotency: if Replicas already 0, skip the PATCH.
	if dep.Spec.Replicas != nil && *dep.Spec.Replicas == 0 {
		// Still apply paused-replicas annotation if a ScaledObject
		// exists (idempotent annotation-set).
		_ = p.applyPausedReplicasAnnotation(ctx, nsName, svc)
		return nil
	}

	// PATCH PodSpec grace period.
	gracePatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"terminationGracePeriodSeconds": gracePeriodSeconds,
				},
			},
		},
	}
	gpBytes, err := json.Marshal(gracePatch)
	if err != nil {
		return errors.WithStack(err)
	}
	// Wrap PATCH in a 3-attempt retry with classified reason on final
	// failure. Reason is discarded here because the caller
	// (reconcileAutoShutdown :fired branch) already classifies from the
	// err shape via classifyPatchError when handling the
	// fireFailedEvent path; preserving the wrapped err preserves the
	// original error semantics for callers that just want a boolean
	// success.
	if _, err := patchDeploymentWithRetry(ctx, p.Cluster, nsName, svc, types.MergePatchType, gpBytes); err != nil {
		return errors.Wrapf(err, "patch grace period %s/%s", nsName, svc)
	}

	// PATCH Deployment replicas=0.
	zeroPatch := []byte(`{"spec":{"replicas":0}}`)
	// 3-attempt retry on the replicas=0 PATCH so a transient K8s API
	// hiccup does not immediately surface as :failed.
	if _, err := patchDeploymentWithRetry(ctx, p.Cluster, nsName, svc, types.MergePatchType, zeroPatch); err != nil {
		return errors.Wrapf(err, "patch replicas=0 %s/%s", nsName, svc)
	}

	// Annotate ScaledObject (if any) with paused-replicas.
	if err := p.applyPausedReplicasAnnotation(ctx, nsName, svc); err != nil {
		return errors.Wrapf(err, "annotate paused-replicas %s/%s", nsName, svc)
	}

	return nil
}

// applyPausedReplicasAnnotation sets `autoscaling.keda.sh/paused-replicas: "0"`
// on the ScaledObject if one exists. Idempotent: skip PATCH if already set.
// Does NOT modify spec.minReplicaCount or spec.maxReplicaCount — those
// belong to the user's chart and the saved-state restore depends on them
// being unchanged.
func (p *Provider) applyPausedReplicasAnnotation(ctx context.Context, ns, name string) error {
	so, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(ctx, name, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil // no ScaledObject; no-op
		}
		return errors.WithStack(err)
	}
	annos := so.GetAnnotations()
	if annos != nil {
		if v, ok := annos[structs.KedaPausedReplicasAnnotation]; ok && v == "0" {
			return nil // already set
		}
	}
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:"0"}}}`, structs.KedaPausedReplicasAnnotation))
	// 3-attempt retry on the dynamic-client PATCH.
	_, err = patchDynamicWithRetry(ctx, p.DynamicClient, scaledObjectGVR, ns, name, types.MergePatchType, patch)
	return errors.WithStack(err)
}

// clearPausedReplicasAnnotation removes the paused-replicas annotation
// via MergePatch null (idempotent on retry).
func (p *Provider) clearPausedReplicasAnnotation(ctx context.Context, ns, name string) error {
	_, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(ctx, name, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil // no ScaledObject; no-op (re-render path is the user's responsibility)
		}
		return errors.WithStack(err)
	}
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:null}}}`, structs.KedaPausedReplicasAnnotation))
	// 3-attempt retry on the dynamic-client PATCH.
	_, err = patchDynamicWithRetry(ctx, p.DynamicClient, scaledObjectGVR, ns, name, types.MergePatchType, patch)
	return errors.WithStack(err)
}

// restoreServiceFromState restores a single service per the saved
// state entry. Pre-flight check: if the user already manually scaled
// the service back up, skip restore for this service and return
// manualDetected=true. Drift merge: saved values win.
func (p *Provider) restoreServiceFromState(ctx context.Context, app string, svc *structs.AppBudgetShutdownStateService) (manualDetected bool, err error) {
	nsName := p.AppNamespace(app)
	dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svc.Name, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return false, nil // service no longer exists; nothing to restore
		}
		return false, errors.WithStack(err)
	}
	currentReplicas := int32(0)
	if dep.Spec.Replicas != nil {
		currentReplicas = *dep.Spec.Replicas
	}
	// Pre-flight: user already manually scaled service back up. Skip
	// restore for this service.
	if currentReplicas > 0 {
		return true, nil
	}
	// Restore replicas.
	target := int32(svc.OriginalScale.Count) //nolint:gosec // user-set replica counts are clamped at K8s level
	if target == 0 && svc.OriginalScale.Replicas > 0 {
		// fallback: if Count was 0 (KEDA-managed at min=0), use last-observed Replicas
		target = int32(svc.OriginalScale.Replicas) //nolint:gosec // see above
	}
	patchObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": target,
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"terminationGracePeriodSeconds": svc.OriginalGracePeriodSeconds,
				},
			},
		},
	}
	patchBytes, err := json.Marshal(patchObj)
	if err != nil {
		return false, errors.WithStack(err)
	}
	// 3-attempt retry on the restore PATCH.
	if _, err := patchDeploymentWithRetry(ctx, p.Cluster, nsName, svc.Name, types.MergePatchType, patchBytes); err != nil {
		return false, errors.Wrapf(err, "patch restore %s/%s", nsName, svc.Name)
	}
	// clearPausedReplicasAnnotation is idempotent (MergePatch null is
	// safe on retry; missing-ScaledObject path returns nil). The
	// PausedReplicasAnnotationSet gate is intentionally absent —
	// :fired did not always flip the flag to true post-PATCH, which
	// would have left KEDA-using services silently uncleaned after
	// `convox budget reset`. Any saved KedaScaledObject triggers the
	// clear regardless of the flag.
	if svc.KedaScaledObject != nil {
		if err := p.clearPausedReplicasAnnotation(ctx, nsName, svc.KedaScaledObject.Name); err != nil {
			fmt.Printf("ns=budget_shutdown at=restore_scaledobject_missing app=%s service=%s err=%q\n", app, svc.Name, err)
		}
	}
	return false, nil
}

// shutdownPlan ties an eligible service to its deployment + scaledobject
// + per-service cost so the accumulator can plan the shutdown order.
type shutdownPlan struct {
	Service     string
	Replicas    int32
	HasKeda     bool
	GraceSecs   int64
	Cost        float64
	LastUpdated time.Time
}

// orderShutdownPlans applies the user-configured shutdown order.
// Two algorithms in 3.24.6: largest-cost (default) and newest. Ties
// broken by lexicographic service name ascending.
func orderShutdownPlans(plans []shutdownPlan, order string) []shutdownPlan {
	sorted := make([]shutdownPlan, len(plans))
	copy(sorted, plans)
	sort.SliceStable(sorted, func(i, j int) bool {
		switch order {
		case "newest":
			if !sorted[i].LastUpdated.Equal(sorted[j].LastUpdated) {
				return sorted[i].LastUpdated.After(sorted[j].LastUpdated)
			}
		default:
			// largest-cost — descending cost, then lex name ascending
			if sorted[i].Cost != sorted[j].Cost {
				return sorted[i].Cost > sorted[j].Cost
			}
		}
		return sorted[i].Service < sorted[j].Service
	})
	return sorted
}

// formatServiceList returns "a,b,c" for event Data payload fields
// (snake_case keys, comma-separated values on the wire).
func formatServiceList(svcs []string) string {
	return strings.Join(svcs, ",")
}

// computeManifestSha256 returns a deterministic hex-encoded SHA-256
// over the eligible service set plus budget config — used to detect
// drift between shutdown and restore.
func computeManifestSha256(eligibleSvcs []string, capUsd float64, atCapAction string) string {
	h := sha256.New()
	for _, s := range eligibleSvcs {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write([]byte(strconv.FormatFloat(capUsd, 'f', 2, 64)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(atCapAction))
	return hex.EncodeToString(h.Sum(nil))
}

// AppBudgetShutdownStateGet returns the shutdown-state annotation for
// an app, or (nil, nil) when no annotation is present. Returned errors
// indicate the namespace lookup failed; corrupt annotations surface as
// (nil, error) so the CLI can render a "state corrupt" diagnostic
// instead of a misleading "no banner" message.
//
// Read-only — does not mutate cluster state. Used by `convox budget show`
// to render the ARMED/ACTIVE/RECOVERED/FAILED banner above the JSON
// payload.
func (p *Provider) AppBudgetShutdownStateGet(app string) (*structs.AppBudgetShutdownState, error) {
	ctx := p.Context()
	if ctx == nil {
		ctx = context.TODO()
	}
	nsName := p.AppNamespace(app)
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil, errors.WithStack(structs.ErrNotFound("app not found: %s", app))
		}
		return nil, errors.WithStack(err)
	}
	state, parseErr := readBudgetShutdownStateAnnotation(ns.Annotations)
	if parseErr != nil {
		return nil, parseErr
	}
	if state == nil {
		return nil, nil
	}

	// Aggregate the separate dismissed-banner annotation into the
	// read-side view. The dismissed annotation is set by
	// writeRecoveryBannerDismissedAnnotation (this file) and persists
	// independently of the shutdown-state annotation. Surfacing it on
	// the same struct lets Console read both via one SDK call and
	// suppress the RECOVERED banner across page reloads. Errors on
	// the dismissed-annotation read are non-fatal — the field stays
	// nil and the user falls back to in-session-only suppression. Log
	// the parse error at structured-stdout severity so an operator
	// chasing a "banner won't dismiss" report has a diagnostic trail;
	// corrupt-annotation is admin-trusted territory (kubectl annotate)
	// so this is rare.
	dismissedAt, dismErr := readRecoveryBannerDismissedAnnotation(ns.Annotations)
	if dismErr != nil {
		fmt.Printf("ns=budget_shutdown at=dismiss_annotation_parse_failed app=%s error=%q\n", app, dismErr)
	}
	if dismErr == nil && dismissedAt != nil {
		state.RecoveryBannerDismissedAt = dismissedAt
	}

	return state, nil
}

// AppBudgetSimulate runs a dry-run shutdown simulation. Reads current
// state, computes eligibility + ordering + estimated savings, fires
// :simulated event with dry_run=true, returns the simulation result.
// Does NOT modify cluster state.
func (p *Provider) AppBudgetSimulate(app string) (*structs.AppBudgetSimulationResult, error) {
	ctx := p.Context()
	if ctx == nil {
		ctx = context.TODO()
	}

	cfg, _, err := p.AppBudgetGet(app)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.WithStack(structs.ErrBadRequest("no budget configured for app %s", app))
	}

	manifest, _, err := p.releaseManifestForApp(app)
	if err != nil {
		return nil, errors.Wrapf(err, "release manifest for app %s", app)
	}

	plan, err := p.computeShutdownPlanForApp(ctx, app, manifest, cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	now := time.Now().UTC()

	wouldShutDownNames := make([]string, 0, len(plan.ordered))
	var totalCostPerHour float64
	for _, sp := range plan.ordered {
		wouldShutDownNames = append(wouldShutDownNames, sp.Service)
		totalCostPerHour += sp.Cost
	}

	result := &structs.AppBudgetSimulationResult{
		App:                          app,
		AtCapAction:                  cfg.AtCapAction,
		WebhookUrl:                   plan.webhookUrl,
		NotifyBeforeMinutes:          plan.notifyBeforeMinutes,
		ShutdownGracePeriod:          plan.shutdownGracePeriod.String(),
		ShutdownOrder:                plan.shutdownOrder,
		RecoveryMode:                 plan.recoveryMode,
		Eligibility:                  plan.eligibility,
		WouldShutDownServices:        wouldShutDownNames,
		WouldShutDownCount:           len(wouldShutDownNames),
		EstimatedCostSavedUsdPerHour: totalCostPerHour,
		SimulatedAt:                  now,
	}

	tickID := generateShutdownTickID(now)
	actor := p.auditActor()
	_ = ctx
	data := universalEventData(actor, tickID, true, cfg.MonthlyCapUsd, plan.spendUsd)
	data["app"] = app
	data["would_shut_down_services"] = formatServiceList(wouldShutDownNames)
	data["would_shut_down_count"] = strconv.Itoa(len(wouldShutDownNames))
	data["estimated_cost_saved_usd_per_hour"] = strconv.FormatFloat(totalCostPerHour, 'f', 2, 64)
	data["shutdown_order"] = plan.shutdownOrder
	data["recovery_mode"] = plan.recoveryMode
	data["notify_before_minutes"] = strconv.Itoa(plan.notifyBeforeMinutes)
	data["simulated_at"] = now.Format(time.RFC3339)
	_ = p.EventSend(shutdownEventName("simulated"), structs.EventSendOptions{Data: data})
	return result, nil
}

// AppBudgetDismissRecovery dismisses the sticky recovery banner. Wraps
// AppBudgetDismissRecoveryWithResult and discards the status value to
// preserve the legacy SDK contract.
func (p *Provider) AppBudgetDismissRecovery(app, ackBy string) error {
	_, err := p.AppBudgetDismissRecoveryWithResult(app, ackBy)
	return err
}

// AppBudgetDismissRecoveryWithResult is the dismiss-recovery path with
// three return statuses:
//
//   - status="dismissed"        : a recovery banner was active; now dismissed
//   - status="already-dismissed": a banner exists but was previously dismissed
//   - status="no-banner"        : no recovery banner is active for this app
//
// Idempotent — repeated calls return "already-dismissed" without writing.
// Banner presence is determined by the shutdown-state annotation having a
// non-zero RestoredAt: a recovery banner is shown post-restore until the
// annotation GCs (one tick after RestoredAt + tick interval, or earlier
// via dismiss-recovery).
func (p *Provider) AppBudgetDismissRecoveryWithResult(app, ackBy string) (*structs.AppBudgetDismissRecoveryResult, error) {
	// Extend the per-app lock surface to dismiss. Without this lock,
	// two concurrent dismiss clicks both observe existing=nil and both
	// write the annotation, producing duplicate :dismissed events with
	// idempotent=false. Same pattern as the accumulator-coordination
	// surface enumerated at budget_app_lock.go.
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()

	ctx := p.Context()
	if ctx == nil {
		ctx = context.TODO()
	}
	ackBy = sanitizeAckBy(ackBy)

	nsName := p.AppNamespace(app)
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil, errors.WithStack(structs.ErrNotFound("app not found: %s", app))
		}
		return nil, errors.WithStack(err)
	}

	// Determine whether a recovery banner is presently shown. Banner
	// shows when shutdown-state has a non-zero RestoredAt (i.e. we
	// post-restored from a fired shutdown). A flap-suppressed-until
	// carry-over annotation alone does NOT show a banner — only the
	// main shutdown-state's RestoredAt does.
	state, _ := readBudgetShutdownStateAnnotation(ns.Annotations)
	bannerActive := state != nil && state.RestoredAt != nil && !state.RestoredAt.IsZero()

	now := time.Now().UTC()
	existing, _ := readRecoveryBannerDismissedAnnotation(ns.Annotations)

	if !bannerActive && existing == nil {
		// No banner present + nothing to dismiss.
		return &structs.AppBudgetDismissRecoveryResult{App: app, Status: structs.BudgetDismissRecoveryStatusNoBanner}, nil
	}
	if existing != nil {
		// Already dismissed — idempotent no-op. Audit event still fires.
		_ = p.fireDismissedEvent(ctx, app, ackBy, *existing, true)
		return &structs.AppBudgetDismissRecoveryResult{App: app, Status: structs.BudgetDismissRecoveryStatusAlreadyDismissed}, nil
	}
	if err := p.writeRecoveryBannerDismissedAnnotation(ctx, app, now); err != nil {
		return nil, err
	}
	_ = p.fireDismissedEvent(ctx, app, ackBy, now, false)
	return &structs.AppBudgetDismissRecoveryResult{App: app, Status: structs.BudgetDismissRecoveryStatusDismissed}, nil
}

// fireDismissedEvent emits the audit-only `:dismissed` event for the
// dismiss-recovery action. Not one of the lifecycle events; separate
// observability hook.
func (p *Provider) fireDismissedEvent(ctx context.Context, app, ackBy string, dismissedAt time.Time, idempotent bool) error {
	_ = ctx
	tickID := generateShutdownTickID(time.Now())
	actor := ackBy
	if actor == "" || actor == "unknown" {
		actor = "system"
	}
	data := universalEventData(actor, tickID, false, 0, 0)
	data["app"] = app
	data["dismissed_at"] = dismissedAt.Format(time.RFC3339)
	data["idempotent"] = strconv.FormatBool(idempotent)
	return p.EventSend(shutdownEventName("dismissed"), structs.EventSendOptions{Data: data})
}

// AppBudgetResetWithOptions extends AppBudgetReset to honor the
// --force-clear-cooldown flag. When ForceClearCooldown is true, the
// carry-over cooldown annotation is also deleted (CanAdmin gate
// enforced server-side at the controller layer, not here).
//
// Annotation handling checklist:
//  1. budget-state:                        CLEAR (existing AppBudgetReset)
//  2. budget-shutdown-state:               UNCONDITIONAL DELETE
//  3. budget-flap-suppressed-until:        PRESERVE (or DELETE w/ force flag)
//  4. budget-recovery-banner-dismissed:    optional (clear so banner re-shows)
//  5. budget-flap-suppress-fired-at:       DELETE (if cooldown cleared)
//
// Holds the per-app lock for the FULL duration of the function
// (breaker-clear AND restoreFromAnnotation + annotation delete) so a
// concurrent accumulator tick cannot acquire the lock between the two
// stages, observe the still-present armed shutdown-state annotation,
// and fire its own :cancelled / :restored event before our
// restoreFromAnnotation emit lands. The inner reset routine is split
// into appBudgetResetLocked (lock-already-held variant) so we acquire
// once at the outer scope.
func (p *Provider) AppBudgetResetWithOptions(app, ackBy string, opts structs.AppBudgetResetOptions) error {
	// Sanitize at outer scope so restoreFromAnnotation receives the
	// canonical form. The inner appBudgetResetLocked path also calls
	// sanitizeAckBy() but pass-by-value semantics meant the outer ackBy
	// used by the restore stage stayed unsanitized. Idempotent —
	// re-sanitizing an already-sanitized string returns the same value.
	ackBy = sanitizeAckBy(ackBy)

	ctx := p.Context()
	if ctx == nil {
		ctx = context.TODO()
	}

	// Hold the per-app advisory lock across all reset stages so
	// restoreFromAnnotation's emits and the unconditional annotation
	// delete are atomic with the breaker-clear. The accumulator's
	// reconcileAutoShutdown also acquires this lock; a concurrent tick
	// queues until our full critical section completes.
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()

	// Clear (1) budget-state + breaker via the lock-already-held helper
	// (we already hold the lock at the outer scope).
	if err := p.appBudgetResetLocked(app, ackBy); err != nil {
		return err
	}

	// Handle (2) budget-shutdown-state via restore-or-unconditional-delete.
	nsName := p.AppNamespace(app)
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	state, parseErr := readBudgetShutdownStateAnnotation(ns.Annotations)
	switch {
	case parseErr != nil:
		// Corrupt-annotation case — unconditionally delete.
		schemaVer := -1
		if raw, ok := ns.Annotations[structs.BudgetShutdownStateAnnotation]; ok {
			var probe struct {
				SchemaVersion int `json:"schemaVersion"`
			}
			_ = json.Unmarshal([]byte(raw), &probe)
			schemaVer = probe.SchemaVersion
		}
		fmt.Printf("ns=budget_reset at=reset_state_corrupt_deleted app=%s schema_version=%d action=annotation_force_deleted\n", app, schemaVer)
		_ = p.deleteBudgetShutdownStateAnnotation(ctx, app)
	case state != nil:
		// Restore + delete annotation.
		if err := p.restoreFromAnnotation(ctx, app, ackBy, state, "reset"); err != nil {
			fmt.Printf("ns=budget_reset at=restore_failed app=%s err=%q\n", app, err)
		}
		_ = p.deleteBudgetShutdownStateAnnotation(ctx, app)
	}

	// Cooldown carry-over (3 + 5).
	if opts.ForceClearCooldown {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressedUntilAnnotation)
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressFiredAtAnnotation)
	}

	// Recovery-banner annotation. We clear it on a fresh
	// restore so the recovery banner is shown for the new restore;
	// keep it intact otherwise.
	if state != nil && state.RestoredAt == nil {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetRecoveryBannerDismissedAnnotation)
	}

	return nil
}

// restoreFromAnnotation runs the per-service restore loop with pre-
// flight check. Events fire AFTER the loop completes (batched), not
// per-iteration.
//
// Reads the live cfg + baseState before emitting so the universal
// cap_usd / spend_usd fields carry real values rather than 0. The
// :cancelled payload carries armed_at / expected_shutdown_at. :failed
// and :restored remain mutually exclusive — :failed only fires when
// len(failed) > 0 AND nothing succeeded; partial-shutdown reports
// succeeded count via partial_state.
func (p *Provider) restoreFromAnnotation(ctx context.Context, app, ackBy string, state *structs.AppBudgetShutdownState, trigger string) error {
	if state == nil {
		return nil
	}
	now := time.Now().UTC()

	// Load live cfg + baseState so universal cap_usd / spend_usd values
	// are populated. Best-effort — emit with 0 fallback if read fails.
	cfg, baseState, _ := p.AppBudgetGet(app)

	// Pre-flight loop.
	manualDetected := []string{}
	restoredOK := []string{}
	failed := []string{}
	keda := 0
	deploymentOnly := 0
	for i := range state.Services {
		svc := &state.Services[i]
		md, err := p.restoreServiceFromState(ctx, app, svc)
		if err != nil {
			fmt.Printf("ns=budget_shutdown at=restore_failed app=%s service=%s err=%q\n", app, svc.Name, err)
			failed = append(failed, svc.Name)
			continue
		}
		if md {
			manualDetected = append(manualDetected, svc.Name)
			continue
		}
		restoredOK = append(restoredOK, svc.Name)
		if svc.KedaScaledObject != nil {
			keda++
		} else {
			deploymentOnly++
		}
	}

	// Determine which event to fire.
	wasArmedOnly := state.ArmedAt != nil && !state.ArmedAt.IsZero() && (state.ShutdownAt == nil || state.ShutdownAt.IsZero())

	tickID := state.ShutdownTickId
	if tickID == "" {
		tickID = generateShutdownTickID(now)
	}
	actor := ackBy
	if actor == "" || actor == "unknown" {
		actor = "system"
	}

	if wasArmedOnly && len(manualDetected) > 0 {
		// Rich :cancelled payload with cap/spend + armed_at /
		// expected_shutdown_at.
		data := universalEventData(actor, tickID, false, capUsdFor(cfg), spendUsdFor(baseState))
		data["app"] = app
		data["cancelled_at"] = now.Format(time.RFC3339)
		data["cancel_reason"] = "manual-detected"
		data["eligible_services"] = formatServiceList(manualDetected)
		// armed_at + expected_shutdown_at on :cancelled.
		if state.ArmedAt != nil {
			data["armed_at"] = state.ArmedAt.UTC().Format(time.RFC3339)
			expected := state.ArmedAt.Add(time.Duration(structs.BudgetDefaultNotifyBeforeMinutes) * time.Minute)
			data["expected_shutdown_at"] = expected.UTC().Format(time.RFC3339)
		}
		_ = p.EventSend(shutdownEventName("cancelled"), structs.EventSendOptions{Data: data})
	} else if len(restoredOK) > 0 || len(manualDetected) > 0 {
		// :restored payload carries cap_usd + spend_usd.
		data := universalEventData(actor, tickID, false, capUsdFor(cfg), spendUsdFor(baseState))
		data["app"] = app
		data["restored_services"] = formatServiceList(append(append([]string{}, restoredOK...), manualDetected...))
		data["restored_count"] = strconv.Itoa(len(restoredOK) + len(manualDetected))
		// recovery_at (not restored_at) is the wire field name.
		data["recovery_at"] = now.Format(time.RFC3339)
		data["recovery_trigger"] = trigger
		if len(manualDetected) > 0 && len(restoredOK) == 0 {
			data["recovery_trigger"] = "manual-detected"
		}
		data["restored_to_keda"] = strconv.Itoa(keda)
		data["restored_to_deployment"] = strconv.Itoa(deploymentOnly)
		data["drift_detected"] = "false"
		flapUntil := now.Add(structs.BudgetFlapCooldown)
		data["flap_suppressed_until"] = flapUntil.UTC().Format(time.RFC3339)
		// Surface final_spend_usd for downstream cost reconciliation.
		if baseState != nil {
			data["final_spend_usd"] = strconv.FormatFloat(baseState.CurrentMonthSpendUsd, 'f', 2, 64)
		}
		_ = p.EventSend(shutdownEventName("restored"), structs.EventSendOptions{Data: data})
		// Persist cooldown carry-over so future ticks suppress flap re-arm.
		_ = p.writeFlapSuppressedUntilAnnotation(ctx, app, flapUntil)
	}

	if len(failed) > 0 {
		// :failed payload carries cap_usd + spend_usd.
		data := universalEventData("system", tickID, false, capUsdFor(cfg), spendUsdFor(baseState))
		data["app"] = app
		data["failed_services"] = formatServiceList(failed)
		data["failed_at"] = now.Format(time.RFC3339)
		data["failure_reason"] = structs.BudgetShutdownReasonK8sApiFailure
		data["partial_state"] = strconv.Itoa(len(restoredOK))
		data["retry_count"] = strconv.Itoa(budgetShutdownPatchRetries)
		_ = p.EventSend(shutdownEventName("failed"), structs.EventSendOptions{Data: data})
	}

	return nil
}

// auditActor returns the actor identity from the request-scoped JWT
// or "system" when no actor is in context (accumulator-driven path).
// The returned value is what lands in the `actor` field of every
// lifecycle event payload.
//
// Per D.3 (commit f68fe4db8 2026-04-22): Provider.ContextActor() reads the
// JWT user claim from p.ctx and falls back to "unknown" when no actor is
// available. We map "unknown" -> "system" here so :armed/:fired payloads
// (always tick-driven, never JWT-bound) carry the canonical "system"
// actor. CLI-driven paths (simulate, dismiss-recovery, reset) override
// this with their own ackBy value at the call site.
func (p *Provider) auditActor() string {
	a := p.ContextActor()
	if a == "" || a == "unknown" {
		return "system"
	}
	return a
}

// shutdownPlanResult captures the eligibility set + ordered plan +
// per-service cost map for a single app at a single tick. Used by both
// the simulate path and the actual shutdown trigger path (so the
// :armed and :simulated payloads share derivation).
type shutdownPlanResult struct {
	ordered             []shutdownPlan
	eligibility         []structs.AppBudgetSimulationEligibility
	webhookUrl          string
	notifyBeforeMinutes int
	shutdownGracePeriod time.Duration
	shutdownOrder       string
	recoveryMode        string
	manifestSha         string
	spendUsd            float64
}

// computeShutdownPlanForApp builds the eligibility list + ordering
// from the app's current manifest + budget config + observed deployments.
// Used by simulate (read-only) AND tick-time arm/fire decisions
// (where the result feeds the state annotation write).
func (p *Provider) computeShutdownPlanForApp(ctx context.Context, app string, m *manifest.Manifest, cfg *structs.AppBudget) (*shutdownPlanResult, error) {
	if m == nil {
		return nil, errors.WithStack(fmt.Errorf("no manifest for app %s", app))
	}
	res := &shutdownPlanResult{
		webhookUrl:          m.Budget.AtCapWebhookUrl,
		notifyBeforeMinutes: m.Budget.NotifyBeforeMinutes,
		shutdownOrder:       m.Budget.ShutdownOrder,
		recoveryMode:        m.Budget.RecoveryMode,
	}
	if res.notifyBeforeMinutes <= 0 {
		res.notifyBeforeMinutes = structs.BudgetDefaultNotifyBeforeMinutes
	}
	gracePeriod := m.Budget.ShutdownGracePeriod
	if gracePeriod == "" {
		gracePeriod = structs.BudgetDefaultShutdownGracePeriod
	}
	d, err := time.ParseDuration(gracePeriod)
	if err != nil {
		return nil, errors.Wrapf(err, "parse shutdownGracePeriod %q", gracePeriod)
	}
	res.shutdownGracePeriod = d
	if res.shutdownOrder == "" {
		res.shutdownOrder = structs.BudgetDefaultShutdownOrder
	}
	if res.recoveryMode == "" {
		res.recoveryMode = structs.BudgetDefaultRecoveryMode
	}

	exempt := map[string]bool{}
	for _, s := range m.Budget.NeverAutoShutdown {
		exempt[s] = true
	}

	// Read current spend from existing state annotation for the simulate
	// path to populate cap_usd / spend_usd in the :simulated event payload.
	if _, st, err := p.AppBudgetGet(app); err == nil && st != nil {
		res.spendUsd = st.CurrentMonthSpendUsd
	}

	// Per-service cost lookup. AppCost returns SpendUsd per service
	// covering the current month-to-date; we convert that to a per-hour
	// rate by dividing by elapsed hours since MonthStart so largest-cost
	// shutdown ordering operates on instantaneous burn rather than total
	// monthly spend (a service that ran for 1h at $10/h should rank above
	// a service that ran for 100h at $1/h even though their MTD totals
	// match). Lookup is best-effort: a transient AppCost error keeps cost=0
	// for the rest of this tick rather than failing the simulate path.
	costByService := map[string]float64{}
	if cost, err := p.AppCost(app); err == nil && cost != nil {
		hours := time.Since(cost.MonthStart).Hours()
		if hours <= 0 {
			hours = 1
		}
		for _, line := range cost.Breakdown {
			costByService[line.Service] = line.SpendUsd / hours
		}
	}

	nsName := p.AppNamespace(app)
	plans := []shutdownPlan{}
	eligibility := []structs.AppBudgetSimulationEligibility{}
	eligibleNames := []string{}
	for i := range m.Services {
		svc := &m.Services[i]
		if svc.Agent.Enabled {
			eligibility = append(eligibility, structs.AppBudgetSimulationEligibility{
				Service: svc.Name, Eligible: false, Reason: "agent service (DaemonSet)",
			})
			continue
		}
		if exempt[svc.Name] {
			eligibility = append(eligibility, structs.AppBudgetSimulationEligibility{
				Service: svc.Name, Eligible: false, Reason: "in neverAutoShutdown",
			})
			continue
		}
		dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svc.Name, am.GetOptions{})
		if err != nil {
			eligibility = append(eligibility, structs.AppBudgetSimulationEligibility{
				Service: svc.Name, Eligible: false, Reason: "no deployment yet (pending first deploy)",
			})
			continue
		}
		replicas := int32(0)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		hasKeda := false
		if _, gerr := p.DynamicClient.Resource(scaledObjectGVR).Namespace(nsName).Get(ctx, svc.Name, am.GetOptions{}); gerr == nil {
			hasKeda = true
		}
		grace := int64(30)
		if dep.Spec.Template.Spec.TerminationGracePeriodSeconds != nil {
			grace = *dep.Spec.Template.Spec.TerminationGracePeriodSeconds
		}
		var lastUpdated time.Time
		if dep.Annotations != nil {
			if v, ok := dep.Annotations["atom.lastUpdated"]; ok {
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					lastUpdated = t
				}
			}
		}
		// Per-service cost — looked up from AppCost.Breakdown above
		// (best-effort; falls back to 0 when the cost lookup hit a
		// transient error or the service has no observed spend yet).
		cost := costByService[svc.Name]
		plans = append(plans, shutdownPlan{
			Service:     svc.Name,
			Replicas:    replicas,
			HasKeda:     hasKeda,
			GraceSecs:   grace,
			Cost:        cost,
			LastUpdated: lastUpdated,
		})
		eligibility = append(eligibility, structs.AppBudgetSimulationEligibility{
			Service:        svc.Name,
			Eligible:       true,
			Replicas:       int(replicas),
			CostUsdPerHour: cost,
		})
		eligibleNames = append(eligibleNames, svc.Name)
	}

	res.ordered = orderShutdownPlans(plans, res.shutdownOrder)
	res.eligibility = eligibility
	if cfg != nil {
		res.manifestSha = computeManifestSha256(eligibleNames, cfg.MonthlyCapUsd, cfg.AtCapAction)
	}
	return res, nil
}

// releaseManifestForApp returns the current release's manifest for the
// app, or an error if no release has been promoted. Wraps
// common.AppManifest for ergonomic call-site access.
func (p *Provider) releaseManifestForApp(app string) (*manifest.Manifest, *structs.Release, error) {
	return common.AppManifest(p, app)
}

// runStaleAnnotationGC removes shutdown-state, flap-suppressed, and
// banner-dismissed annotations whose terminal-state timestamp passed
// more than one tick ago. Runs UNCONDITIONALLY on every tick (NOT
// gated on cost_tracking_enable) so kubectl drift cleans up.
//
// Sigils:
//   - state.RestoredAt > 1 tick ago    → delete state annotation; carry-over flap; clear dismissed
//   - state.ExpiredAt > 1 tick ago     → delete state annotation; carry-over flap; clear dismissed
//   - flap-suppressed-until expired    → delete flap-suppressed AND flap-fired-at TOGETHER
//   - dismissed annotation             → cleared together with state annotation
//
// Invariant: FlapSuppressFiredAt and FlapSuppressedUntil are always
// written and cleared together. The accumulator dedup-write at
// budget_auto_shutdown.go gated on the live state's FlapSuppressedUntil
// being set; the GC and Reset paths clear both in the same block. No
// orphan annotation can persist past one tick.
func (p *Provider) runStaleAnnotationGC(ctx context.Context, app string, tickInterval time.Duration) error {
	nsName := p.AppNamespace(app)
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	now := time.Now().UTC()

	// (a) main shutdown-state GC
	state, parseErr := readBudgetShutdownStateAnnotation(ns.Annotations)
	if parseErr == nil && state != nil {
		var terminalAt *time.Time
		if state.RestoredAt != nil && !state.RestoredAt.IsZero() {
			terminalAt = state.RestoredAt
		} else if state.ExpiredAt != nil && !state.ExpiredAt.IsZero() {
			terminalAt = state.ExpiredAt
		}
		if terminalAt != nil && terminalAt.Before(now.Add(-tickInterval)) {
			if state.FlapSuppressedUntil != nil && state.FlapSuppressedUntil.After(now) {
				_ = p.writeFlapSuppressedUntilAnnotation(ctx, app, *state.FlapSuppressedUntil)
			}
			_ = p.deleteBudgetShutdownStateAnnotation(ctx, app)
			// Clear the dismissed banner annotation alongside the
			// shutdown-state annotation. Without this clear, cycle-N's
			// dismiss timestamp leaks into cycle-N+1's RECOVERED state;
			// the GET-side aggregation surfaces the stale timestamp;
			// Vue suppresses the new banner silently.
			if err := p.deleteNamespaceAnnotation(ctx, app, structs.BudgetRecoveryBannerDismissedAnnotation); err != nil {
				fmt.Printf("ns=budget_shutdown at=warn kind=stale_gc_dismissed_annotation_delete app=%s err=%q\n", app, err.Error())
			}
		}
	}

	// (b) flap-suppressed-until carry-over GC
	flap, _ := readFlapSuppressedUntilAnnotation(ns.Annotations)
	if flap != nil && flap.Before(now.Add(-tickInterval)) {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressedUntilAnnotation)
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressFiredAtAnnotation)
	}
	return nil
}
