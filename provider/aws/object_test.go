package aws_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/convox/convox/pkg/mock/aws"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectDeleteSuccess(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("HeadObject", &s3.HeadObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, nil)

		s3api.On("DeleteObject", &s3.DeleteObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, nil)

		err := p.ObjectDelete("app1", "key1")
		require.NoError(t, err)
	})
}

func TestObjectDeleteHeadObjectError(t *testing.T) {
	err := errors.New("Object Not Found")

	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("HeadObject", &s3.HeadObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, err)

		s3api.On("DeleteObject", &s3.DeleteObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, nil)

		err := p.ObjectDelete("app1", "key1")
		assert.Equal(t, "Object Not Found", err.Error())
	})
}

func TestObjectDeleteError(t *testing.T) {
	err := errors.New("Object Not Found")

	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("HeadObject", &s3.HeadObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, nil)

		s3api.On("DeleteObject", &s3.DeleteObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, err)

		err := p.ObjectDelete("app1", "key1")
		assert.Equal(t, "Object Not Found", err.Error())
	})
}

func TestObjectDeleteNotFoundError(t *testing.T) {
	err := awserr.New("NotFound", "Object not found", nil)

	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("HeadObject", &s3.HeadObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, nil)

		s3api.On("DeleteObject", &s3.DeleteObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, err)

		err := p.ObjectDelete("app1", "key1")
		assert.Equal(t, "NotFound: Object not found", err.Error())
	})
}

func TestObjectFetchSuccess(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("GetObject", &s3.GetObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(strings.NewReader("hello world")),
		}, nil)

		_, err := p.ObjectFetch("app1", "key1")
		require.NoError(t, err)
	})
}

func TestObjectFetchError(t *testing.T) {
	err := errors.New("Object Not Found")

	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("GetObject", &s3.GetObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, err)

		_, err := p.ObjectFetch("app1", "key1")
		assert.Equal(t, "Object Not Found", err.Error())
	})
}

func TestObjectFetchNoSuchKeyError(t *testing.T) {
	err := awserr.New("NoSuchKey", "Object not found", nil)

	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("GetObject", &s3.GetObjectInput{
			Bucket: awssdk.String("bucket1"),
			Key:    awssdk.String("app1/key1"),
		}).Return(nil, err)

		_, err := p.ObjectFetch("app1", "key1")
		assert.Equal(t, "object not found: key1", err.Error())
	})
}

func TestObjectListSuccess(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("ListObjectsV2", &s3.ListObjectsV2Input{
			Bucket:    awssdk.String("bucket1"),
			Delimiter: awssdk.String("/"),
			Prefix:    awssdk.String("app1/key1"),
		}).Return(&s3.ListObjectsV2Output{
			Contents: []*s3.Object{&s3.Object{Key: awssdk.String("key1")}},
		}, nil)

		res, err := p.ObjectList("app1", "key1")
		require.NoError(t, err)
		assert.Equal(t, []string{"key1"}, res)
	})
}

func TestObjectListError(t *testing.T) {
	err := awserr.New("NoSuchKey", "Object not found", nil)

	testProvider(t, func(p *aws.Provider) {
		s3api := p.S3.(*mocks.S3API)
		s3api.On("ListObjectsV2", &s3.ListObjectsV2Input{
			Bucket:    awssdk.String("bucket1"),
			Delimiter: awssdk.String("/"),
			Prefix:    awssdk.String("app1/key1"),
		}).Return(nil, err)

		_, err := p.ObjectList("app1", "key1")
		assert.Equal(t, "NoSuchKey: Object not found", err.Error())
	})
}

func TestObjectKeyTenantPrefix(t *testing.T) {
	tests := []struct {
		Name  string
		Tid   string
		App   string
		Key   string
		S3Key string
	}{
		{Name: "TidSentinelApp", Tid: "ab12", App: "x-undefined-app-x", Key: "cd34-app2/tmp/x.tgz", S3Key: "ab12-x-undefined-app-x/cd34-app2/tmp/x.tgz"},
		{Name: "TidEmptyApp", Tid: "ab12", App: "", Key: "k", S3Key: "ab12-/k"},
		{Name: "TidApp", Tid: "ab12", App: "app1", Key: "k", S3Key: "ab12-app1/k"},
		{Name: "TidStoredUrlPath", Tid: "ab12", App: "app1", Key: "/build/B1/logs", S3Key: "ab12-app1/build/B1/logs"},
		{Name: "TidDotSegments", Tid: "ab12", App: "app1", Key: "/../cd34-app2/tmp/x.tgz", S3Key: "ab12-app1/cd34-app2/tmp/x.tgz"},
		{Name: "SentinelApp", App: "x-undefined-app-x", Key: "k", S3Key: "k"},
		{Name: "App", App: "app1", Key: "k", S3Key: "app1/k"},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			testProvider(t, func(p *aws.Provider) {
				s3api, ok := p.S3.(*mocks.S3API)
				require.True(t, ok)
				s3api.On("GetObject", &s3.GetObjectInput{
					Bucket: awssdk.String("bucket1"),
					Key:    awssdk.String(test.S3Key),
				}).Return(&s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil)

				pp, ok := p.WithContext(context.WithValue(context.Background(), structs.ConvoxTIDCtxKey, test.Tid)).(*aws.Provider)
				require.True(t, ok)

				_, err := pp.ObjectFetch(test.App, test.Key)
				require.NoError(t, err)
			})
		})
	}
}

func TestObjectListTenantPrefix(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		s3api, ok := p.S3.(*mocks.S3API)
		require.True(t, ok)
		s3api.On("ListObjectsV2", &s3.ListObjectsV2Input{
			Bucket:    awssdk.String("bucket1"),
			Delimiter: awssdk.String("/"),
			Prefix:    awssdk.String("ab12-app1/tmp/"),
		}).Return(&s3.ListObjectsV2Output{}, nil)

		pp, ok := p.WithContext(context.WithValue(context.Background(), structs.ConvoxTIDCtxKey, "ab12")).(*aws.Provider)
		require.True(t, ok)

		_, err := pp.ObjectList("app1", "tmp/")
		require.NoError(t, err)
	})
}
