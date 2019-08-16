package main

import (
	"os"

	"github.com/convox/convox/pkg/cli"
)

var (
	version = "dev"
)

func main() {
	c := cli.New("cx", version)

	os.Exit(c.Execute(os.Args[1:]))
}
