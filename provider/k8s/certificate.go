package k8s

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (p *Provider) CertificateApply(app, service string, port int, id string) error {
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

func (p *Provider) CertificateGenerate(domains []string) (*structs.Certificate, error) {
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

func (p *Provider) CertificateList() (structs.Certificates, error) {
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

		for _, c := range certs {
			cs = append(cs, c)
		}
	}

	return cs, nil
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

func (p *Provider) certificateFromSecret(s *ac.Secret) (*structs.Certificate, error) {
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
	ss, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).List(context.TODO(), am.ListOptions{
		FieldSelector: "type=kubernetes.io/tls",
	})
	if err != nil {
		return errors.WithStack(err)
	}

	for _, s := range ss.Items {
		if strings.Contains(s.Name, "-domains") {
			return p.renewCertificate(p.AppNamespace(app), s.Name)
		}
	}

	return nil
}

func (p *Provider) renewCertificate(namespace, name string) error {
	if err := p.Cluster.CoreV1().Secrets(p.Namespace).Delete(context.TODO(), name, am.DeleteOptions{}); err != nil {
		return errors.WithStack(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}

	return p.DynamicClient.Resource(gvr).
		Namespace(namespace).Delete(context.TODO(), name, am.DeleteOptions{})
}
