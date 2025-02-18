//go:build tools
// +build tools

package convox

import (
	_ "github.com/crazy-max/xgo"
	_ "github.com/gobuffalo/packr/packr"
	_ "github.com/goware/modvendor"
	_ "github.com/vektra/mockery/cmd/mockery"
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/conversion-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/defaulter-gen"
	_ "k8s.io/code-generator/cmd/go-to-protobuf"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "k8s.io/code-generator/cmd/register-gen"
)
