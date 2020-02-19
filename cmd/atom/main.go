package main

import (
	"fmt"
	"os"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	"k8s.io/client-go/rest"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}

func run() error {
	common.InitializeKlog()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	if err := atom.Initialize(); err != nil {
		return err
	}

	ac, err := atom.NewController(cfg)
	if err != nil {
		return err
	}

	ac.Run()

	return nil
}
