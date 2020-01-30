package azure

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/v1/operationalinsights"
	"github.com/Azure/azure-storage-file-go/azfile"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/convox/convox/pkg/elastic"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"k8s.io/apimachinery/pkg/util/runtime"
)

type Provider struct {
	*k8s.Provider

	ClientID       string
	ClientSecret   string
	Region         string
	Registry       string
	ResourceGroup  string
	StorageAccount string
	StorageShare   string
	Subscription   string
	Workspace      string

	elastic          *elastic.Client
	insightLogs      *operationalinsights.QueryClient
	storageDirectory *azfile.DirectoryURL
}

func FromEnv() (*Provider, error) {
	k, err := k8s.FromEnv()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Provider:       k,
		ClientID:       os.Getenv("AZURE_CLIENT_ID"),
		ClientSecret:   os.Getenv("AZURE_CLIENT_SECRET"),
		Region:         os.Getenv("REGION"),
		Registry:       os.Getenv("REGISTRY"),
		ResourceGroup:  os.Getenv("RESOURCE_GROUP"),
		StorageAccount: os.Getenv("STORAGE_ACCOUNT"),
		StorageShare:   os.Getenv("STORAGE_SHARE"),
		Subscription:   os.Getenv("AZURE_SUBSCRIPTION_ID"),
		Workspace:      os.Getenv("WORKSPACE"),
	}

	k.Engine = p

	return p, nil
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	if err := p.initializeAzureServices(); err != nil {
		return err
	}

	if err := p.Provider.Initialize(opts); err != nil {
		return err
	}

	runtime.ErrorHandlers = []func(error){}

	return nil
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.Provider = pp.Provider.WithContext(ctx).(*k8s.Provider)
	return &pp
}

func (p *Provider) initializeAzureServices() error {
	ec, err := elastic.New(os.Getenv("ELASTIC_URL"))
	if err != nil {
		return err
	}

	p.elastic = ec

	il, err := p.azureInsightLogs()
	if err != nil {
		return err
	}

	p.insightLogs = il

	sd, err := p.azureStorageDirectory()
	if err != nil {
		return err
	}

	p.storageDirectory = sd

	return nil
}

func (p *Provider) azureAuthorizer(resource string) (autorest.Authorizer, error) {
	a, err := auth.NewAuthorizerFromEnvironmentWithResource(resource)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (p *Provider) azureInsightLogs() (*operationalinsights.QueryClient, error) {
	a, err := p.azureAuthorizer("https://api.loganalytics.io")
	if err != nil {
		return nil, err
	}

	qc := operationalinsights.NewQueryClient()
	qc.Authorizer = a

	return &qc, nil
}

func (p *Provider) azureStorageDirectory() (*azfile.DirectoryURL, error) {
	k, err := p.azureStorageKey()
	if err != nil {
		return nil, err
	}

	cred, err := azfile.NewSharedKeyCredential(p.StorageAccount, k)
	if err != nil {
		return nil, err
	}

	pipe := azfile.NewPipeline(cred, azfile.PipelineOptions{})

	u, err := url.Parse(fmt.Sprintf("https://%s.file.core.windows.net", p.StorageAccount))
	if err != nil {
		return nil, err
	}

	dir := azfile.NewServiceURL(*u, pipe).NewShareURL(p.StorageShare).NewRootDirectoryURL()

	return &dir, nil
}

func (p *Provider) azureStorageKey() (string, error) {
	ctx := context.Background()

	a, err := p.azureAuthorizer("https://management.azure.com")
	if err != nil {
		return "", err
	}

	ac := storage.NewAccountsClient(p.Subscription)
	ac.Authorizer = a

	res, err := ac.ListKeys(ctx, p.ResourceGroup, p.StorageAccount, storage.Kerb)
	if err != nil {
		return "", err
	}
	if len(*res.Keys) < 1 {
		return "", fmt.Errorf("could not find account key")
	}

	return *(*res.Keys)[0].Value, nil
}
