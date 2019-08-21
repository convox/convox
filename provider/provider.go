package provider

import (
	"fmt"
	"os"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/aws"
	"github.com/convox/convox/provider/k8s"
)

var Mock = &structs.MockProvider{}

func FromEnv() (structs.Provider, error) {
	name := os.Getenv("PROVIDER")

	switch name {
	case "aws":
		return aws.FromEnv()
	case "k8s":
		return k8s.FromEnv()
	// case "local":
	//   return local.FromEnv()
	case "test":
		return Mock, nil
	case "":
		return nil, fmt.Errorf("PROVIDER not set")
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}
