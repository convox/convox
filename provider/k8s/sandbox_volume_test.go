package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const sandboxClaim = `[{"name":"sbx-1","size":"10Gi","mountPath":"/home/convox","seedFromImage":true}]`

func TestProcessRun_VolumeClaimCreatesAndMounts(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	opts := sandboxRunOptions()
	opts.VolumeClaims = options.String(sandboxClaim)

	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts)

	pvc, err := kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Get(context.TODO(), "sbx-1", am.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "true", pvc.Labels[SandboxLabel])
	assert.Equal(t, "sbx-1", pvc.Labels[SandboxIDLabel])
	assert.Equal(t, []ac.PersistentVolumeAccessMode{ac.ReadWriteOnce}, pvc.Spec.AccessModes)
	want := resource.MustParse("10Gi")
	assert.True(t, want.Equal(pvc.Spec.Resources.Requests[ac.ResourceStorage]))
	assert.Nil(t, pvc.Spec.StorageClassName, "no class means the cluster default")

	vol := findVolume(pod.Spec.Volumes, sandboxVolumeName)
	require.NotNil(t, vol)
	assert.Equal(t, "sbx-1", vol.PersistentVolumeClaim.ClaimName)
	var mount *ac.VolumeMount
	for i, m := range pod.Spec.Containers[0].VolumeMounts {
		if m.Name == sandboxVolumeName {
			mount = &pod.Spec.Containers[0].VolumeMounts[i]
		}
	}
	require.NotNil(t, mount)
	assert.Equal(t, "/home/convox", mount.MountPath)

	require.NotNil(t, pod.Spec.SecurityContext)
	require.NotNil(t, pod.Spec.SecurityContext.FSGroup)
	assert.Equal(t, int64(1000), *pod.Spec.SecurityContext.FSGroup)
	assert.Equal(t, ac.FSGroupChangeOnRootMismatch, *pod.Spec.SecurityContext.FSGroupChangePolicy)

	require.Len(t, pod.Spec.InitContainers, 1)
	ic := pod.Spec.InitContainers[0]
	assert.Equal(t, pod.Spec.Containers[0].Image, ic.Image)
	assert.Equal(t, sandboxSeedVolumePath, ic.VolumeMounts[0].MountPath)
	assert.Contains(t, ic.Command[2], `cp -a "$1/." "$2/"`)
	assert.Equal(t, []string{"sandbox-seed", "/home/convox", "/convox-seed"}, ic.Command[3:], "paths are arguments, never part of the script")
}

func TestProcessRun_VolumeClaimReusedAcrossRuns(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	creates := 0
	kk.PrependReactor("create", "persistentvolumeclaims", func(a k8stesting.Action) (bool, runtime.Object, error) {
		creates++
		return false, nil, nil
	})
	opts := sandboxRunOptions()
	opts.VolumeClaims = options.String(sandboxClaim)

	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts)
	require.NoError(t, kk.CoreV1().Pods(p.AppNamespace(sandboxHostApp)).Delete(context.TODO(), pod.Name, am.DeleteOptions{}))
	runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts)

	assert.Equal(t, 1, creates)
}

func TestProcessRun_VolumeClaimWithoutSeedHasNoInitContainer(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	opts := sandboxRunOptions()
	opts.VolumeClaims = options.String(`[{"name":"sbx-1","size":"1Gi","mountPath":"/data","storageClass":"gp3"}]`)

	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts)
	assert.Empty(t, pod.Spec.InitContainers)

	pvc, err := kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Get(context.TODO(), "sbx-1", am.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "gp3", *pvc.Spec.StorageClassName)
}

func TestProcessRun_VolumeClaimRackDefaultStorageClass(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	p.SandboxStorageClass = "sandbox-gp3"
	opts := sandboxRunOptions()
	opts.VolumeClaims = options.String(sandboxClaim)

	runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts)

	pvc, err := kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Get(context.TODO(), "sbx-1", am.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "sandbox-gp3", *pvc.Spec.StorageClassName)
}

func TestProcessRun_VolumeClaimRequiresSandboxLabel(t *testing.T) {
	p, _ := runFlagsProvider(t)
	_, err := p.ProcessRun("app1", "web", structs.ProcessRunOptions{
		Release:      options.String("rel1"),
		VolumeClaims: options.String(sandboxClaim),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for sandbox processes")
}

func TestProcessRun_VolumeClaimRefusesForeignPVC(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	_, err := kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Create(context.TODO(), &ac.PersistentVolumeClaim{
		ObjectMeta: am.ObjectMeta{Name: "sbx-1"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	opts := sandboxRunOptions()
	opts.VolumeClaims = options.String(sandboxClaim)
	_, err = p.ProcessRun(sandboxHostApp, "sbx", opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a sandbox volume")
}

func TestParseSandboxVolumeClaims(t *testing.T) {
	cases := []struct {
		name, raw, err string
	}{
		{"bad json", `{`, "invalid volume claims"},
		{"two claims", `[{"name":"a","size":"1Gi","mountPath":"/a"},{"name":"b","size":"1Gi","mountPath":"/b"}]`, "at most one"},
		{"bad name", `[{"name":"Bad_Name","size":"1Gi","mountPath":"/a"}]`, "invalid volume claim name"},
		{"bad size", `[{"name":"a","size":"lots","mountPath":"/a"}]`, "invalid volume claim size"},
		{"zero size", `[{"name":"a","size":"0","mountPath":"/a"}]`, "invalid volume claim size"},
		{"relative path", `[{"name":"a","size":"1Gi","mountPath":"home"}]`, "invalid volume claim mount path"},
		{"root path", `[{"name":"a","size":"1Gi","mountPath":"/"}]`, "invalid volume claim mount path"},
		{"unclean path", `[{"name":"a","size":"1Gi","mountPath":"/a/../b"}]`, "invalid volume claim mount path"},
		{"bad class", `[{"name":"a","size":"1Gi","mountPath":"/a","storageClass":"Bad Class"}]`, "invalid storage class"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSandboxVolumeClaims(tc.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err)
		})
	}

	claims, err := parseSandboxVolumeClaims(`[]`)
	require.NoError(t, err)
	assert.Empty(t, claims)
}

func TestProcessVolumeClaimDelete(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	opts := sandboxRunOptions()
	opts.VolumeClaims = options.String(sandboxClaim)
	runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts)

	require.NoError(t, p.ProcessVolumeClaimDelete(sandboxHostApp, "sbx-1"))
	_, err := kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Get(context.TODO(), "sbx-1", am.GetOptions{})
	require.Error(t, err)

	require.NoError(t, p.ProcessVolumeClaimDelete(sandboxHostApp, "sbx-1"), "deleting a missing claim is a no-op")
}

func TestProcessVolumeClaimDeleteRefusesForeignPVC(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	_, err := kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Create(context.TODO(), &ac.PersistentVolumeClaim{
		ObjectMeta: am.ObjectMeta{Name: "data"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.ProcessVolumeClaimDelete(sandboxHostApp, "data")
	require.Error(t, err)

	_, err = kk.CoreV1().PersistentVolumeClaims(p.AppNamespace(sandboxHostApp)).Get(context.TODO(), "data", am.GetOptions{})
	require.NoError(t, err, "a non-sandbox claim is never deleted")
}

func TestProcessRun_NoVolumeClaimsLeavesSpecAlone(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxRunOptions())
	assert.Nil(t, findVolume(pod.Spec.Volumes, sandboxVolumeName))
	assert.Empty(t, pod.Spec.InitContainers)
	if pod.Spec.SecurityContext != nil {
		assert.Nil(t, pod.Spec.SecurityContext.FSGroup)
	}
}

func findVolume(vs []ac.Volume, name string) *ac.Volume {
	for i := range vs {
		if vs[i].Name == name {
			return &vs[i]
		}
	}
	return nil
}

func TestProcessRun_VolumeClaimHardening(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	ns := "rack1-" + sandboxHostApp

	for raw, msg := range map[string]string{
		`[{"name":"v","size":"1Gi","mountPath":"/home/a;id"}]`:     "mount path",
		`[{"name":"v","size":"1Gi","mountPath":"/home/a b"}]`:      "mount path",
		`[{"name":"v","size":"101Gi","mountPath":"/home/convox"}]`: "limit",
	} {
		_, err := parseSandboxVolumeClaims(raw)
		require.Error(t, err, raw)
		assert.Contains(t, err.Error(), msg, raw)
	}

	o := sandboxRunOptions()
	o.VolumeClaims = options.String(`[{"name":"shared","size":"1Gi","mountPath":"/home/convox"}]`)
	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", o)
	require.NoError(t, kk.CoreV1().Pods(ns).Delete(context.TODO(), pod.Name, am.DeleteOptions{}))

	other := sandboxRunOptions()
	other.Labels = options.String(SandboxLabel + "=true," + SandboxIDLabel + "=sbx-2")
	other.VolumeClaims = o.VolumeClaims
	_, err := p.ProcessRun(sandboxHostApp, "sbx", other)
	require.Error(t, err, "one sandbox cannot mount another's workspace")
	assert.Contains(t, err.Error(), "another sandbox")

	noID := sandboxRunOptions()
	noID.Labels = options.String(SandboxLabel + "=true")
	noID.VolumeClaims = o.VolumeClaims
	_, err = p.ProcessRun(sandboxHostApp, "sbx", noID)
	require.Error(t, err)
}

func TestProcessRun_SandboxLabelMustBeTrue(t *testing.T) {
	p, kk := runFlagsProvider(t)
	_, err := p.ProcessRun("app1", "web", structs.ProcessRunOptions{Release: options.String("rel1"), Labels: options.String(SandboxLabel + "=false")})
	require.Error(t, err, "any other value would match neither the isolation nor the sandbox policies")
	_ = kk
}
