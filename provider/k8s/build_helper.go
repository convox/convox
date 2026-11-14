package k8s

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	buildHelperPort     = 1234
	buildHelperDeadline = 6 * time.Hour
	buildHelperCertLife = 24 * time.Hour
)

// buildHelper is a buildkitd pod that builds one architecture of a build on a node of that architecture.
type buildHelper struct {
	id        string
	arch      string
	name      string
	namespace string
	host      string

	ca, cert, key         []byte
	clientCert, clientKey []byte
}

func (p *Provider) buildNativeArch() string {
	arch := os.Getenv("BUILD_NATIVE_ARCH")
	if p.BuildNodeEnabled != "true" || (arch != "amd64" && arch != "arm64") {
		return ""
	}

	return arch
}

func (p *Provider) buildPlacement(s *ac.PodSpec, arch string) {
	if p.BuildNodeEnabled == "true" {
		exprs := []ac.NodeSelectorRequirement{
			{
				Key:      "convox-build",
				Operator: ac.NodeSelectorOpIn,
				Values:   []string{"true"},
			},
		}
		if arch != "" {
			exprs = append(exprs, ac.NodeSelectorRequirement{
				Key:      "kubernetes.io/arch",
				Operator: ac.NodeSelectorOpIn,
				Values:   []string{arch},
			})
		}
		s.Affinity = &ac.Affinity{
			NodeAffinity: &ac.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &ac.NodeSelector{
					NodeSelectorTerms: []ac.NodeSelectorTerm{
						{
							MatchExpressions: exprs,
						},
					},
				},
			},
		}
	}

	s.Tolerations = append(s.Tolerations, ac.Toleration{
		Key:      "dedicated",
		Operator: ac.TolerationOpExists,
		Effect:   ac.TaintEffectNoSchedule,
	})
}

func (p *Provider) buildHelperPrepare(app, id, arch string) (*buildHelper, error) {
	h := &buildHelper{
		id:        strings.ToLower(id),
		arch:      arch,
		namespace: p.processBuildNamespace(app),
	}
	h.name = fmt.Sprintf("build-%s-%s", h.id, arch)
	h.host = fmt.Sprintf("%s.%s.svc.cluster.local", h.name, h.namespace)

	if err := h.certificates(time.Now().UTC()); err != nil {
		return nil, errors.WithStack(err)
	}

	return h, nil
}

func (h *buildHelper) env() map[string]string {
	return map[string]string{
		"BUILD_HELPER_ADDR": fmt.Sprintf("tcp://%s:%d", h.host, buildHelperPort),
		"BUILD_HELPER_ARCH": h.arch,
		"BUILD_HELPER_CA":   string(h.ca),
		"BUILD_HELPER_CERT": string(h.clientCert),
		"BUILD_HELPER_KEY":  string(h.clientKey),
	}
}

func (h *buildHelper) certificates(now time.Time) error {
	template := func(serial int64) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: "convox build"},
			NotBefore:    now.Add(-5 * time.Minute),
			NotAfter:     now.Add(buildHelperCertLife),
		}
	}

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	ca := template(1)
	ca.IsCA = true
	ca.BasicConstraintsValid = true
	ca.KeyUsage = x509.KeyUsageCertSign

	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	if ca, err = x509.ParseCertificate(der); err != nil {
		return err
	}

	h.ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	leaf := func(cert *x509.Certificate, usage x509.ExtKeyUsage) ([]byte, []byte, error) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, err
		}

		cert.KeyUsage = x509.KeyUsageDigitalSignature
		cert.ExtKeyUsage = []x509.ExtKeyUsage{usage}

		der, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		if err != nil {
			return nil, nil, err
		}

		kder, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, nil, err
		}

		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}), nil
	}

	server := template(2)
	server.DNSNames = []string{h.host}

	if h.cert, h.key, err = leaf(server, x509.ExtKeyUsageServerAuth); err != nil {
		return err
	}

	h.clientCert, h.clientKey, err = leaf(template(3), x509.ExtKeyUsageClientAuth)

	return err
}

// buildHelperCreate ties the helper to the build pod, so it is removed when the build pod is.
func (p *Provider) buildHelperCreate(app, process string, h *buildHelper, opts *structs.ProcessRunOptions) error {
	pd, ns, err := p.buildPod(app, process)
	if err != nil {
		return err
	}
	if pd == nil || ns != h.namespace {
		return fmt.Errorf("build pod %s is not in namespace %s", process, h.namespace)
	}

	meta := func() am.ObjectMeta {
		return am.ObjectMeta{
			Name:      h.name,
			Namespace: ns,
			Labels: map[string]string{
				"app":          app,
				"build":        h.id,
				"rack":         p.Name,
				"service-type": "build",
				"system":       "convox",
				"type":         "build-helper",
			},
			OwnerReferences: []am.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: pd.Name, UID: pd.UID}},
		}
	}

	secret := &ac.Secret{
		ObjectMeta: meta(),
		Data:       map[string][]byte{"ca.crt": h.ca, "tls.crt": h.cert, "tls.key": h.key},
	}
	if _, err := p.Cluster.CoreV1().Secrets(ns).Create(context.TODO(), secret, am.CreateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	service := &ac.Service{
		ObjectMeta: meta(),
		Spec: ac.ServiceSpec{
			Ports:    []ac.ServicePort{{Port: buildHelperPort, TargetPort: intstr.FromInt(buildHelperPort)}},
			Selector: map[string]string{"build": h.id, "type": "build-helper"},
		},
	}
	if _, err := p.Cluster.CoreV1().Services(ns).Create(context.TODO(), service, am.CreateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	s, err := p.podSpecFromRunOptions(app, "build", ns, structs.ProcessRunOptions{
		Cpu:     opts.Cpu,
		Image:   opts.Image,
		IsBuild: true,
		Memory:  opts.Memory,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	pod := &ac.Pod{ObjectMeta: meta()}

	c := &s.Containers[0]
	c.Command = []string{
		"buildkitd",
		"--addr", fmt.Sprintf("tcp://0.0.0.0:%d", buildHelperPort),
		"--tlscacert", "/certs/ca.crt",
		"--tlscert", "/certs/tls.crt",
		"--tlskey", "/certs/tls.key",
	}
	c.SecurityContext = &ac.SecurityContext{SeccompProfile: &ac.SeccompProfile{Type: ac.SeccompProfileTypeUnconfined}}
	if opts.Privileged != nil && *opts.Privileged {
		c.SecurityContext.Privileged = options.Bool(true)
		pod.Annotations = map[string]string{fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", c.Name): "unconfined"}
	}
	c.VolumeMounts = append(c.VolumeMounts, ac.VolumeMount{Name: "certs", MountPath: "/certs", ReadOnly: true})

	s.ActiveDeadlineSeconds = options.Int64(int64(buildHelperDeadline.Seconds()))
	s.Volumes = append(s.Volumes, ac.Volume{
		Name:         "certs",
		VolumeSource: ac.VolumeSource{Secret: &ac.SecretVolumeSource{SecretName: h.name}},
	})

	p.buildPlacement(s, h.arch)

	pod.Spec = *s

	if _, err := p.Cluster.CoreV1().Pods(ns).Create(context.TODO(), pod, am.CreateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
