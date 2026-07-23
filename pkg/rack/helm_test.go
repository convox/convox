package rack

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStuckHelmSecrets(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	allow := convoxHelmReleases("myrack")

	stale := now.Add(-1 * time.Hour)
	fresh := now.Add(-2 * time.Minute)

	secret := func(name, release, ns, status string, created time.Time) helmReleaseSecret {
		return helmReleaseSecret{secretName: name, release: release, namespace: ns, status: status, version: "1", created: created}
	}

	found := []helmReleaseSecret{
		secret("sh.helm.release.v1.aws-lbc.v3", "aws-lbc", "kube-system", "pending-upgrade", stale),
		secret("sh.helm.release.v1.karpenter.v2", "karpenter", "kube-system", "pending-install", stale),
		secret("sh.helm.release.v1.contour.v5", "contour", "myrack-system", "pending-upgrade", stale),
		secret("sh.helm.release.v1.keda.v1", "keda", "keda", "deployed", stale),
		secret("sh.helm.release.v1.vpa.v4", "vpa", "vpa", "superseded", stale),
		secret("sh.helm.release.v1.aws-lbc.v2", "aws-lbc", "kube-system", "failed", stale),
		secret("sh.helm.release.v1.dcgm.v1", "dcgm-exporter", "kube-system", "pending-upgrade", fresh),
		secret("sh.helm.release.v1.keda.v9", "keda", "default", "pending-upgrade", stale),
		secret("sh.helm.release.v1.datadog.v1", "datadog", "monitoring", "pending-upgrade", stale),
		secret("sh.helm.release.v1.contour.v1", "contour", "kube-system", "pending-upgrade", stale),
	}

	got := stuckHelmSecrets(found, allow, now, helmStuckMinAge)

	var names []string
	for _, s := range got {
		names = append(names, s.secretName)
	}

	assert.ElementsMatch(t, []string{
		"sh.helm.release.v1.aws-lbc.v3",
		"sh.helm.release.v1.karpenter.v2",
		"sh.helm.release.v1.contour.v5",
	}, names)
}

func TestStuckHelmSecretsEmpty(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	assert.Empty(t, stuckHelmSecrets(nil, convoxHelmReleases("myrack"), now, helmStuckMinAge))
}
