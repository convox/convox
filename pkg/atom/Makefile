.PHONY: generate mocks

generate:
	$(eval tmp := $(shell mktemp -d))
	bash ../../vendor/k8s.io/code-generator/generate-groups.sh all \
		github.com/convox/convox/pkg/atom/pkg/client \
		github.com/convox/convox/pkg/atom/pkg/apis \
		atom:v1 \
		--output-base $(tmp) \
		--go-header-file ../../hack/boilerplate.txt
	rsync -a $(tmp)/github.com/convox/convox/pkg/atom/pkg .

mocks:
	rm -f mock_Interface.go
	go run ../../vendor/github.com/vektra/mockery/cmd/mockery/mockery.go -inpkg -name Interface
