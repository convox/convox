all: generate

generate:
	# needs k8s.io/code-generator@release-1.11
	env $(GOPATH)/src/k8s.io/code-generator/generate-groups.sh all \
		github.com/convox/convox/pkg/atom/pkg/client \
		github.com/convox/convox/pkg/atom/pkg/apis \
		atom:v1
