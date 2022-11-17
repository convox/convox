package main

import (
	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/provider"
)

func main() {
	p, _ := provider.FromEnv()
	api.NewWithProvider(p)
}
