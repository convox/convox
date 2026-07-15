package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var certLabels = map[string]string{"system": "convox", "type": "letsencrypt-certificate"}

func newTestSecretController(objs ...runtime.Object) (*SecretController, *fake.Clientset) {
	c := fake.NewSimpleClientset(objs...)
	sc := &SecretController{
		Provider: &Provider{
			Cluster:   c,
			Namespace: "rack1",
			ctx:       context.Background(),
			logger:    logger.New("ns=secret-test"),
		},
		log: logger.New("ns=secret-test"),
	}
	return sc, c
}

func mkSecret(ns, name string, labels, ann map[string]string, data map[string][]byte) *ac.Secret {
	return &ac.Secret{
		ObjectMeta: am.ObjectMeta{Namespace: ns, Name: name, Labels: labels, Annotations: ann},
		Data:       data,
	}
}

func patchCount(c *fake.Clientset, ns, name string) int {
	n := 0
	for _, a := range c.Actions() {
		pa, ok := a.(k8stesting.PatchAction)
		if ok && a.GetVerb() == "patch" && a.GetResource().Resource == "secrets" && a.GetNamespace() == ns && pa.GetName() == name {
			n++
		}
	}
	return n
}

func TestSecretSyncCrossAppNoContamination(t *testing.T) {
	app1 := mkSecret("app1", "cert-web", certLabels, map[string]string{}, map[string][]byte{"tls.crt": []byte("A"), "tls.key": []byte("A")})
	app2 := mkSecret("app2", "cert-web", certLabels, map[string]string{AnnotationSecretDataHash: "stale"}, map[string][]byte{"tls.crt": []byte("B"), "tls.key": []byte("B")})
	sc, c := newTestSecretController(app1, app2)
	c.ClearActions()

	require.NotPanics(t, func() { _ = sc.Update(app1, app1) })

	got, err := c.CoreV1().Secrets("app2").Get(context.TODO(), "cert-web", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []byte("B"), got.Data["tls.crt"])
	require.Equal(t, 0, patchCount(c, "app2", "cert-web"))
}

func TestSecretSyncFlowAPropagation(t *testing.T) {
	src := mkSecret("rack1", "cert-abc", certLabels, map[string]string{}, map[string][]byte{"tls.crt": []byte("NEW"), "tls.key": []byte("NEWKEY")})
	cp := mkSecret("myapp", "cert-abc", nil, map[string]string{AnnotationSecretDataHash: "stale"}, map[string][]byte{"tls.crt": []byte("OLD"), "tls.key": []byte("OLDKEY")})
	sc, c := newTestSecretController(src, cp)
	c.ClearActions()

	require.NoError(t, sc.Update(src, src))

	got, err := c.CoreV1().Secrets("myapp").Get(context.TODO(), "cert-abc", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []byte("NEW"), got.Data["tls.crt"])
	require.Equal(t, []byte("NEWKEY"), got.Data["tls.key"])
	want, err := secretDataHash(src)
	require.NoError(t, err)
	require.Equal(t, want, got.Annotations[AnnotationSecretDataHash])
	require.Equal(t, 0, patchCount(c, "rack1", "cert-abc"))
}

func TestSecretSyncNoOpWhenHashMatches(t *testing.T) {
	data := map[string][]byte{"tls.crt": []byte("SAME"), "tls.key": []byte("SAMEKEY")}
	src := mkSecret("rack1", "cert-abc", certLabels, map[string]string{}, data)
	h, err := secretDataHash(src)
	require.NoError(t, err)
	cp := mkSecret("myapp", "cert-abc", nil, map[string]string{AnnotationSecretDataHash: h}, data)
	sc, c := newTestSecretController(src, cp)
	c.ClearActions()

	require.NoError(t, sc.Update(src, src))

	require.Equal(t, 0, patchCount(c, "myapp", "cert-abc"))
}

func TestSecretDataHashDeterministic(t *testing.T) {
	s := &ac.Secret{Data: map[string][]byte{
		"tls.key": []byte("KEYDATA"),
		"tls.crt": []byte("CRTDATA"),
		"ca.crt":  []byte("CADATA"),
	}}

	first, err := secretDataHash(s)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	for i := 0; i < 200; i++ {
		got, err := secretDataHash(s)
		require.NoError(t, err)
		require.Equal(t, first, got)
	}

	s2 := &ac.Secret{Data: map[string][]byte{
		"ca.crt":  []byte("CADATA"),
		"tls.crt": []byte("CRTDATA"),
		"tls.key": []byte("KEYDATA"),
	}}
	got2, err := secretDataHash(s2)
	require.NoError(t, err)
	require.Equal(t, first, got2)
}

func TestSecretSyncNilAnnotationsTarget(t *testing.T) {
	src := mkSecret("rack1", "cert-abc", certLabels, map[string]string{}, map[string][]byte{"tls.crt": []byte("NEW"), "tls.key": []byte("NEW")})
	cp := mkSecret("myapp", "cert-abc", nil, nil, map[string][]byte{"tls.crt": []byte("OLD"), "tls.key": []byte("OLD")})
	sc, c := newTestSecretController(src, cp)

	require.NotPanics(t, func() { _ = sc.Update(src, src) })

	got, err := c.CoreV1().Secrets("myapp").Get(context.TODO(), "cert-abc", am.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, got.Annotations[AnnotationSecretDataHash])
}

func TestSecretSyncListErrorNoPanic(t *testing.T) {
	src := mkSecret("rack1", "cert-abc", certLabels, map[string]string{}, map[string][]byte{"tls.crt": []byte("X")})
	sc, c := newTestSecretController(src)
	c.PrependReactor("list", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("boom")
	})

	require.NotPanics(t, func() { _ = sc.Update(src, src) })
}

func TestSecretListOptionsSelector(t *testing.T) {
	sc, _ := newTestSecretController()
	opts := &am.ListOptions{}
	sc.ListOptions(opts)
	require.Contains(t, opts.LabelSelector, "letsencrypt-certificate")
	require.Contains(t, opts.LabelSelector, "state")
}
