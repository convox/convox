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

const shutdownTickIDPrefix = "tick-"

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

const budgetShutdownPatchRetries = 3

func shutdownEventName(suffix string) string {
	return "app:budget:auto-shutdown:" + suffix
}

// cap_usd emitted as int (no decimals); spend_usd keeps decimals
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

func generateShutdownTickID(now time.Time) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // UUIDv4 version+variant bits
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s%s-%s", shutdownTickIDPrefix, now.UTC().Format("2006-01-02T15:04:05Z"), hex.EncodeToString(b[:]))
}

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

func (p *Provider) writeBudgetShutdownStateAnnotation(ctx context.Context, app string, s *structs.AppBudgetShutdownState, resourceVersion string) error {
	nsName := p.AppNamespace(app)

	// nil RecoveryBannerDismissedAt before marshal; stored in separate annotation
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

func (p *Provider) writeFlapSuppressedUntilAnnotation(ctx context.Context, app string, until time.Time) error {
	return p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetFlapSuppressedUntilAnnotation, until.UTC().Format(time.RFC3339))
}

func (p *Provider) writeRecoveryBannerDismissedAnnotation(ctx context.Context, app string, at time.Time) error {
	return p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetRecoveryBannerDismissedAnnotation, at.UTC().Format(time.RFC3339))
}

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

func (p *Provider) shutdownService(ctx context.Context, app, svc string, gracePeriodSeconds int64) error {
	nsName := p.AppNamespace(app)
	dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svc, am.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "get deployment %s/%s", nsName, svc)
	}

	if dep.Spec.Replicas != nil && *dep.Spec.Replicas == 0 {
		_ = p.applyPausedReplicasAnnotation(ctx, nsName, svc)
		return nil
	}

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
	if _, err := patchDeploymentWithRetry(ctx, p.Cluster, nsName, svc, types.MergePatchType, gpBytes); err != nil {
		return errors.Wrapf(err, "patch grace period %s/%s", nsName, svc)
	}

	zeroPatch := []byte(`{"spec":{"replicas":0}}`)
	if _, err := patchDeploymentWithRetry(ctx, p.Cluster, nsName, svc, types.MergePatchType, zeroPatch); err != nil {
		return errors.Wrapf(err, "patch replicas=0 %s/%s", nsName, svc)
	}

	if err := p.applyPausedReplicasAnnotation(ctx, nsName, svc); err != nil {
		return errors.Wrapf(err, "annotate paused-replicas %s/%s", nsName, svc)
	}

	return nil
}

func (p *Provider) applyPausedReplicasAnnotation(ctx context.Context, ns, name string) error {
	so, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(ctx, name, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	annos := so.GetAnnotations()
	if annos != nil {
		if v, ok := annos[structs.KedaPausedReplicasAnnotation]; ok && v == "0" {
			return nil
		}
	}
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:"0"}}}`, structs.KedaPausedReplicasAnnotation))
	_, err = patchDynamicWithRetry(ctx, p.DynamicClient, scaledObjectGVR, ns, name, types.MergePatchType, patch)
	return errors.WithStack(err)
}

func (p *Provider) clearPausedReplicasAnnotation(ctx context.Context, ns, name string) error {
	_, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace(ns).Get(ctx, name, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:null}}}`, structs.KedaPausedReplicasAnnotation))
	_, err = patchDynamicWithRetry(ctx, p.DynamicClient, scaledObjectGVR, ns, name, types.MergePatchType, patch)
	return errors.WithStack(err)
}

func (p *Provider) restoreServiceFromState(ctx context.Context, app string, svc *structs.AppBudgetShutdownStateService) (manualDetected bool, err error) {
	nsName := p.AppNamespace(app)
	dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svc.Name, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return false, nil
		}
		return false, errors.WithStack(err)
	}
	currentReplicas := int32(0)
	if dep.Spec.Replicas != nil {
		currentReplicas = *dep.Spec.Replicas
	}
	if currentReplicas > 0 {
		return true, nil
	}
	target := int32(svc.OriginalScale.Count) //nolint:gosec
	if target == 0 && svc.OriginalScale.Replicas > 0 {
		target = int32(svc.OriginalScale.Replicas) //nolint:gosec
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
	if _, err := patchDeploymentWithRetry(ctx, p.Cluster, nsName, svc.Name, types.MergePatchType, patchBytes); err != nil {
		return false, errors.Wrapf(err, "patch restore %s/%s", nsName, svc.Name)
	}
	if svc.KedaScaledObject != nil {
		if err := p.clearPausedReplicasAnnotation(ctx, nsName, svc.KedaScaledObject.Name); err != nil {
			fmt.Printf("ns=budget_shutdown at=restore_scaledobject_missing app=%s service=%s err=%q\n", app, svc.Name, err)
		}
	}
	return false, nil
}

type shutdownPlan struct {
	Service     string
	Replicas    int32
	HasKeda     bool
	GraceSecs   int64
	Cost        float64
	LastUpdated time.Time
}

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
			if sorted[i].Cost != sorted[j].Cost {
				return sorted[i].Cost > sorted[j].Cost
			}
		}
		return sorted[i].Service < sorted[j].Service
	})
	return sorted
}

func formatServiceList(svcs []string) string {
	return strings.Join(svcs, ",")
}

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

	// aggregate dismissed-banner annotation into read-side view
	dismissedAt, dismErr := readRecoveryBannerDismissedAnnotation(ns.Annotations)
	if dismErr != nil {
		fmt.Printf("ns=budget_shutdown at=dismiss_annotation_parse_failed app=%s error=%q\n", app, dismErr)
	}
	if dismErr == nil && dismissedAt != nil {
		state.RecoveryBannerDismissedAt = dismissedAt
	}

	return state, nil
}

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

func (p *Provider) AppBudgetDismissRecovery(app, ackBy string) error {
	_, err := p.AppBudgetDismissRecoveryWithResult(app, ackBy)
	return err
}

func (p *Provider) AppBudgetDismissRecoveryWithResult(app, ackBy string) (*structs.AppBudgetDismissRecoveryResult, error) {
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

	state, _ := readBudgetShutdownStateAnnotation(ns.Annotations)
	bannerActive := state != nil && state.RestoredAt != nil && !state.RestoredAt.IsZero()

	now := time.Now().UTC()
	existing, _ := readRecoveryBannerDismissedAnnotation(ns.Annotations)

	if !bannerActive && existing == nil {
		return &structs.AppBudgetDismissRecoveryResult{App: app, Status: structs.BudgetDismissRecoveryStatusNoBanner}, nil
	}
	if existing != nil {
		_ = p.fireDismissedEvent(ctx, app, ackBy, *existing, true)
		return &structs.AppBudgetDismissRecoveryResult{App: app, Status: structs.BudgetDismissRecoveryStatusAlreadyDismissed}, nil
	}
	if err := p.writeRecoveryBannerDismissedAnnotation(ctx, app, now); err != nil {
		return nil, err
	}
	_ = p.fireDismissedEvent(ctx, app, ackBy, now, false)
	return &structs.AppBudgetDismissRecoveryResult{App: app, Status: structs.BudgetDismissRecoveryStatusDismissed}, nil
}

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

func (p *Provider) AppBudgetResetWithOptions(app, ackBy string, opts structs.AppBudgetResetOptions) error {
	ackBy = sanitizeAckBy(ackBy)

	ctx := p.Context()
	if ctx == nil {
		ctx = context.TODO()
	}

	// lock across all reset stages to serialize with accumulator tick
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()

	if err := p.appBudgetResetLocked(app, ackBy, opts); err != nil {
		return err
	}

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
		if err := p.restoreFromAnnotation(ctx, app, ackBy, state, "reset"); err != nil {
			fmt.Printf("ns=budget_reset at=restore_failed app=%s err=%q\n", app, err)
		}
		_ = p.deleteBudgetShutdownStateAnnotation(ctx, app)
	}

	if opts.ForceClearCooldown {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressedUntilAnnotation)
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressFiredAtAnnotation)
	}

	// clear dismissed so new restore shows the banner
	if state != nil && state.RestoredAt == nil {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetRecoveryBannerDismissedAnnotation)
	}

	return nil
}

func (p *Provider) restoreFromAnnotation(ctx context.Context, app, ackBy string, state *structs.AppBudgetShutdownState, trigger string) error {
	if state == nil {
		return nil
	}
	now := time.Now().UTC()

	cfg, baseState, _ := p.AppBudgetGet(app)

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
		data := universalEventData(actor, tickID, false, capUsdFor(cfg), spendUsdFor(baseState))
		data["app"] = app
		data["cancelled_at"] = now.Format(time.RFC3339)
		data["cancel_reason"] = "manual-detected"
		data["eligible_services"] = formatServiceList(manualDetected)
		if state.ArmedAt != nil {
			data["armed_at"] = state.ArmedAt.UTC().Format(time.RFC3339)
			expected := state.ArmedAt.Add(time.Duration(structs.BudgetDefaultNotifyBeforeMinutes) * time.Minute)
			data["expected_shutdown_at"] = expected.UTC().Format(time.RFC3339)
		}
		_ = p.EventSend(shutdownEventName("cancelled"), structs.EventSendOptions{Data: data})
	} else if len(restoredOK) > 0 || len(manualDetected) > 0 {
		data := universalEventData(actor, tickID, false, capUsdFor(cfg), spendUsdFor(baseState))
		data["app"] = app
		data["restored_services"] = formatServiceList(append(append([]string{}, restoredOK...), manualDetected...))
		data["restored_count"] = strconv.Itoa(len(restoredOK) + len(manualDetected))
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
		if baseState != nil {
			data["final_spend_usd"] = strconv.FormatFloat(baseState.CurrentMonthSpendUsd, 'f', 2, 64)
		}
		_ = p.EventSend(shutdownEventName("restored"), structs.EventSendOptions{Data: data})
		_ = p.writeFlapSuppressedUntilAnnotation(ctx, app, flapUntil)
	}

	if len(failed) > 0 {
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

func (p *Provider) auditActor() string {
	a := p.ContextActor()
	if a == "" || a == "unknown" {
		return "system"
	}
	return a
}

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

	if _, st, err := p.AppBudgetGet(app); err == nil && st != nil {
		res.spendUsd = st.CurrentMonthSpendUsd
	}

	// convert MTD spend to per-hour rate for largest-cost ordering
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

func (p *Provider) releaseManifestForApp(app string) (*manifest.Manifest, *structs.Release, error) {
	return common.AppManifest(p, app)
}

// runs unconditionally (not gated on cost_tracking_enable) so kubectl drift cleans up
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
			// clear dismissed alongside state to prevent stale timestamp leaking into next cycle
			if err := p.deleteNamespaceAnnotation(ctx, app, structs.BudgetRecoveryBannerDismissedAnnotation); err != nil {
				fmt.Printf("ns=budget_shutdown at=warn kind=stale_gc_dismissed_annotation_delete app=%s err=%q\n", app, err.Error())
			}
		}
	}

	flap, _ := readFlapSuppressedUntilAnnotation(ns.Annotations)
	if flap != nil && flap.Before(now.Add(-tickInterval)) {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressedUntilAnnotation)
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetFlapSuppressFiredAtAnnotation)
	}
	return nil
}
