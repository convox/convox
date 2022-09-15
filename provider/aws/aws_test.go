package aws_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func testProvider(t *testing.T, fn func(*aws.Provider)) {
	a := &atom.MockInterface{}
	c := fake.NewSimpleClientset()
	cc := cvfake.NewSimpleClientset()
	mc := metricfake.NewSimpleClientset()

	_, err := c.CoreV1().Namespaces().Create(
		context.TODO(), &ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name: "ns1",
				Labels: map[string]string{
					"app":    "system",
					"rack":   "rack1",
					"system": "convox",
					"type":   "rack",
				},
				UID: "uid1",
			},
		},
		am.CreateOptions{},
	)
	require.NoError(t, err)

	p := &k8s.Provider{
		Atom:          a,
		Cluster:       c,
		Convox:        cc,
		Domain:        "domain1",
		Engine:        &mock.TestEngine{},
		MetricsClient: mc,
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
	}

	err = p.Initialize(structs.ProviderOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "test"}}, am.CreateOptions{})
	require.NoError(t, err)

	os.Setenv("NAMESPACE", "test")

	pp := &aws.Provider{
		Provider: p,
		Bucket:   "bucket1",
		Region:   "us-east-1",

		CloudWatchLogs: &mocks.CloudWatchLogsAPI{},
		ECR:            &mocks.ECRAPI{},
		CloudFormation: &mocks.CloudFormationAPI{},
		S3:             &mocks.S3API{},
		SQS:            &mocks.SQSAPI{},
	}

	fn(pp)

	a.AssertExpectations(t)
}

func requireYamlFixture(t *testing.T, d1 []byte, filename string) {
	r1, err := reformatYaml(d1)
	require.NoError(t, err)

	d2, err := os.ReadFile(filepath.Join("../k8s/testdata", filename))
	require.NoError(t, err)

	r2, err := reformatYaml(d2)
	require.NoError(t, err)

	require.Equal(t, string(r2), string(r1))
}

func reformatYaml(data []byte) ([]byte, error) {
	rms := [][]byte{}

	parts := bytes.Split(data, []byte("---\n"))

	for _, part := range parts {
		var v interface{}

		if err := yaml.Unmarshal(part, &v); err != nil {
			return nil, errors.WithStack(err)
		}

		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		rms = append(rms, data)
	}

	return bytes.Join(rms, []byte("---\n")), nil
}
