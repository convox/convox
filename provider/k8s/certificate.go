package k8s

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	cmapiutil "github.com/cert-manager/cert-manager/pkg/api/util"
	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	amv1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

const (
	LETSENCRYPT_CONFIG = "letsencrypt-config"
	ISSUER_LETSENCRYPT = "letsencrypt"
)

func (*Provider) CertificateApply(_, _ string, _ int, _ string) error {
	return errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) CertificateCreate(pub, key string, opts structs.CertificateCreateOptions) (*structs.Certificate, error) {
	s, err := p.Cluster.CoreV1().Secrets(p.Namespace).Create(
		context.TODO(),
		&ac.Secret{
			ObjectMeta: am.ObjectMeta{
				GenerateName: "cert-",
				Labels: map[string]string{
					"system": "convox",
					"rack":   p.Name,
					"type":   "certificate",
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte(fmt.Sprintf("%s\n%s", pub, common.DefaultString(opts.Chain, ""))),
				"tls.key": []byte(key),
			},
			Type: "kubernetes.io/tls",
		},
		am.CreateOptions{},
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c, err := p.certificateFromSecret(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c, nil
}

func (p *Provider) CertificateDelete(id string) error {
	if err := p.Cluster.CoreV1().Secrets(p.Namespace).Delete(context.TODO(), id, am.DeleteOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) CertificateGenerate(domains []string, opts structs.CertificateGenerateOptions) (*structs.Certificate, error) {
	if opts.Issuer == nil {
		return p.certificateGenerateSelfSigned(domains)
	} else if *opts.Issuer == ISSUER_LETSENCRYPT {
		return p.certificateGenerateLetsencrypt(domains, opts)
	}
	return nil, fmt.Errorf("invalid issuer")
}

func (p *Provider) certificateGenerateSelfSigned(domains []string) (*structs.Certificate, error) {
	switch len(domains) {
	case 0:
		return nil, errors.WithStack(fmt.Errorf("must specify a domain"))
	case 1:
	default:
		return nil, errors.WithStack(fmt.Errorf("must specify only one domain"))
	}

	c, err := common.CertificateSelfSigned(domains[0])
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pub, key, err := common.CertificateParts(c)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return p.CertificateCreate(string(pub), string(key), structs.CertificateCreateOptions{})
}

func (p *Provider) certificateGenerateLetsencrypt(domains []string, opts structs.CertificateGenerateOptions) (*structs.Certificate, error) {
	if len(domains) == 0 {
		return nil, errors.WithStack(fmt.Errorf("must specify a domain"))
	}

	if len(domains) > 10 {
		return nil, errors.WithStack(fmt.Errorf("can not specify more than 10 domain name"))
	}

	duration := 4320 * time.Hour // 6 months
	if opts.Duration != nil {
		var err error
		duration, err = time.ParseDuration(*opts.Duration)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %s", err)
		}
	}

	for i := range domains {
		domains[i] = strings.TrimSpace(domains[i])
	}

	h := sha1.New()
	_, err := h.Write([]byte(strings.Join(domains, "-")))
	if err != nil {
		return nil, fmt.Errorf("failed generate hash: %s", err)
	}

	domainHash := hex.EncodeToString(h.Sum(nil))
	certId := fmt.Sprintf("cert-%s", domainHash)

	// create letsencrypt cert request
	_, err = p.CertManagerClient.CertmanagerV1().Certificates(p.Namespace).Create(p.ctx, &cmapi.Certificate{
		ObjectMeta: am.ObjectMeta{
			Name:      certId,
			Namespace: p.Namespace,
		},
		Spec: cmapi.CertificateSpec{
			IssuerRef: cmmeta.ObjectReference{
				Group: "cert-manager.io",
				Kind:  "ClusterIssuer",
				Name:  "letsencrypt",
			},
			DNSNames:   domains,
			SecretName: certId,
			Duration: &am.Duration{
				Duration: duration,
			},
			Usages: []cmapi.KeyUsage{
				cmapi.UsageDigitalSignature,
				cmapi.UsageKeyEncipherment,
			},
			SecretTemplate: &cmapi.CertificateSecretTemplate{
				Labels: map[string]string{
					"system": "convox",
					"type":   "letsencrypt-certificate",
				},
			},
		},
	}, am.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return &structs.Certificate{
		Id:      certId,
		Domain:  domains[0],
		Domains: domains,
	}, nil
}

func (p *Provider) CertificateList(opts structs.CertificateListOptions) (structs.Certificates, error) {
	if opts.Generated != nil && *opts.Generated {
		return p.generatedCertificateList()
	}
	ns, err := p.Cluster.CoreV1().Namespaces().List(context.TODO(), am.ListOptions{
		LabelSelector: fmt.Sprintf("system=convox,rack=%s", p.Name),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cs := structs.Certificates{}
	for _, n := range ns.Items {
		certs, err := p.certFromNamespace(n)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		cs = append(cs, certs...)
	}

	return cs, nil
}

func (p *Provider) generatedCertificateList() (structs.Certificates, error) {
	certs, err := p.certFromNamespace(ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: p.Namespace,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	certMap := map[string]structs.Certificate{}
	for i := range certs {
		certMap[certs[i].Id] = certs[i]
	}

	certList, err := p.CertManagerClient.CertmanagerV1().Certificates(p.Namespace).List(p.ctx, am.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range certList.Items {
		conds := certList.Items[i].Status.Conditions
		ready := false
		for j := range conds {
			if conds[j].Type == cmapi.CertificateConditionReady && conds[j].Status == cmmeta.ConditionTrue {
				ready = true
			}
		}
		if _, has := certMap[certList.Items[i].Name]; !has && !ready {
			certs = append(certs, structs.Certificate{
				Id:      certList.Items[i].Name,
				Domain:  certList.Items[i].Spec.CommonName,
				Domains: certList.Items[i].Spec.DNSNames,
				Status:  "Not Ready",
			})
		}
	}

	return certs, nil
}

func (p *Provider) certFromNamespace(ns ac.Namespace) (structs.Certificates, error) {
	ss, err := p.Cluster.CoreV1().Secrets(ns.Name).List(context.TODO(), am.ListOptions{
		FieldSelector: "type=kubernetes.io/tls",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cs := structs.Certificates{}

	for _, s := range ss.Items {
		c, err := p.certificateFromSecret(&s)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		cs = append(cs, *c)
	}

	return cs, nil
}

func (*Provider) certificateFromSecret(s *ac.Secret) (*structs.Certificate, error) {
	crt, ok := s.Data["tls.crt"]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("invalid certificate: %s", s.ObjectMeta.Name))
	}

	pb, _ := pem.Decode(crt)

	if pb.Type != "CERTIFICATE" {
		return nil, errors.WithStack(fmt.Errorf("invalid certificate: %s", s.ObjectMeta.Name))
	}

	cs, err := x509.ParseCertificates(pb.Bytes)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(cs) < 1 {
		return nil, errors.WithStack(fmt.Errorf("invalid certificate: %s", s.ObjectMeta.Name))
	}

	c := &structs.Certificate{
		Id:         s.ObjectMeta.Name,
		Domain:     cs[0].Subject.CommonName,
		Domains:    cs[0].DNSNames,
		Expiration: cs[0].NotAfter,
	}

	return c, nil
}

func (p *Provider) CertificateRenew(app string) error {
	certs, err := p.CertManagerClient.CertmanagerV1().Certificates(p.AppNamespace(app)).List(context.TODO(), am.ListOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	for _, cert := range certs.Items {
		if strings.Contains(cert.Name, "-domains") {
			if err := p.renewCertificate(&cert); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Provider) renewCertificate(crt *cmapi.Certificate) error {
	cmapiutil.SetCertificateCondition(crt, crt.Generation, cmapi.CertificateConditionIssuing, cmmeta.ConditionTrue, "ManuallyTriggered", "Certificate re-issuance manually triggered")
	_, err := p.CertManagerClient.CertmanagerV1().Certificates(crt.Namespace).UpdateStatus(context.TODO(), crt, am.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to trigger issuance of Certificate %s: %v", crt.Name, err)
	}
	return nil
}

func (p *Provider) LetsEncryptConfigApply(config structs.LetsEncryptConfig) error {
	config.Defaults()
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	sObj := &corev1.SecretApplyConfiguration{
		TypeMetaApplyConfiguration: amv1.TypeMetaApplyConfiguration{
			Kind:       options.String("Secret"),
			APIVersion: options.String("v1"),
		},
		ObjectMetaApplyConfiguration: &amv1.ObjectMetaApplyConfiguration{
			Name: options.String(LETSENCRYPT_CONFIG),
			Labels: map[string]string{
				"system": "convox",
				"rack":   p.Name,
			},
		},
		Data: map[string][]byte{
			"config": data,
		},
	}
	if _, err = p.Cluster.CoreV1().Secrets(CERT_MANAGER_NAMESPACE).Apply(context.TODO(), sObj, am.ApplyOptions{
		FieldManager: "convox-system",
	}); err != nil {
		return err
	}

	return p.applySystemTemplate("cert-manager-letsencrypt", map[string]interface{}{
		"Config": config,
	})
}

func (p *Provider) LetsEncryptConfigGet() (*structs.LetsEncryptConfig, error) {
	c := &structs.LetsEncryptConfig{}
	c.Defaults()
	s, err := p.Cluster.CoreV1().Secrets(CERT_MANAGER_NAMESPACE).Get(
		context.TODO(), LETSENCRYPT_CONFIG, am.GetOptions{},
	)
	if err != nil {
		if kerr.IsNotFound(err) {
			return c, nil
		}
		return nil, errors.WithStack(err)
	}

	if err := json.Unmarshal(s.Data["config"], c); err != nil {
		return nil, err
	}

	return c, nil
}
