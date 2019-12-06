package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/convox/convox/pkg/atom"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}

func run() error {
	// hack to make glog stop complaining about flag parsing
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	_ = fs.Parse([]string{})
	flag.CommandLine = fs
	runtime.ErrorHandlers = []func(error){}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	ac, err := atom.NewController(cfg)
	if err != nil {
		return err
	}

	ac.Run()

	return nil
}
