all: generate

generate:
	# go get -u k8s.io/code-generator/...
	bash ../../vendor/k8s.io/code-generator/generate-groups.sh all \
		github.com/convox/convox/provider/k8s/pkg/client \
		github.com/convox/convox/provider/k8s/pkg/apis \
		convox:v1
