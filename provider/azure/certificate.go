package azure

// import (
// 	"fmt"

// 	gc "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
// 	"github.com/convox/convox/pkg/structs"
// 	am "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// func (p *Provider) CertificateGenerate(domains []string) (*structs.Certificate, error) {
// 	switch len(domains) {
// 	case 0:
// 		return nil, fmt.Errorf("must specify a domain")
// 	case 1:
// 	default:
// 		return nil, fmt.Errorf("must specify only one domain for gcp managed certificates")
// 	}

// 	gmc, err := p.gkeManagedCertsClient()
// 	if err != nil {
// 		return nil, err
// 	}

// 	cert := &gc.ManagedCertificate{
// 		ObjectMeta: am.ObjectMeta{
// 			GenerateName: "managed-",
// 			Namespace:    p.Namespace,
// 		},
// 		Spec: gc.ManagedCertificateSpec{
// 			Domains: domains,
// 		},
// 		Status: gc.ManagedCertificateStatus{
// 			DomainStatus: []gc.DomainStatus{},
// 		},
// 	}

// 	c, err := gmc.NetworkingV1beta1().ManagedCertificates(p.Namespace).Create(cert)
// 	if err != nil {
// 		return nil, err
// 	}

// 	fmt.Printf("c: %+v\n", c)

// 	return &structs.Certificate{}, nil
// }
