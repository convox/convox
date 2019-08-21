all: all build generate release

all: build

build:
	go install .

generate:
	# needs k8s.io/code-generator@release-1.11
	$(GOPATH)/src/k8s.io/code-generator/generate-groups.sh all \
		github.com/convox/convox/provider/aws/pkg/client \
		github.com/convox/convox/provider/aws/pkg/apis \
		convox:v1
