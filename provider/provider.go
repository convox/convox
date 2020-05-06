package provider

import (
	"fmt"
	"os"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
	"github.com/convox/convox/provider/azure"
	"github.com/convox/convox/provider/do"
	"github.com/convox/convox/provider/gcp"
	"github.com/convox/convox/provider/local"
	"github.com/convox/convox/provider/metal"
)

var Mock = &structs.MockProvider{}

func FromEnv() (structs.Provider, error) {
	name := os.Getenv("PROVIDER")

	switch name {
	case "aws":
		return aws.FromEnv()
	case "azure":
		return azure.FromEnv()
	case "do":
		return do.FromEnv()
	case "gcp":
		return gcp.FromEnv()
	case "local":
		return local.FromEnv()
	case "metal":
		return metal.FromEnv()
	case "test":
		return Mock, nil
	case "":
		return nil, fmt.Errorf("PROVIDER not set")
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func Valid(slug string) bool {
	switch slug {
	case "aws":
		return true
	case "azure":
		return true
	case "do":
		return true
	case "gcp":
		return true
	case "local":
		return true
	case "metal":
		return true
	default:
		return false
	}
}
