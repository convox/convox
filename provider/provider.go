package provider

import (
	"fmt"
	"os"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
)

var Mock = &structs.MockProvider{}

// FromEnv returns a new Provider from env vars
func FromEnv() (structs.Provider, error) {
	return FromName(os.Getenv("PROVIDER"))
}

func FromName(name string) (structs.Provider, error) {
	switch name {
	// case "aws":
	//   return aws.FromEnv()
	case "k8s":
		return k8s.New(os.Getenv("NAMESPACE"))
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
