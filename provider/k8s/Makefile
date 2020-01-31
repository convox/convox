all: generate

generate:
	$(eval tmp := $(shell mktemp -d))
	bash ../../vendor/k8s.io/code-generator/generate-groups.sh all \
		github.com/convox/convox/provider/k8s/pkg/client \
		github.com/convox/convox/provider/k8s/pkg/apis \
		convox:v1 \
		--output-base $(tmp) \
		--go-header-file ../../hack/boilerplate.txt
	rsync -a $(tmp)/github.com/convox/convox/provider/k8s/pkg .
