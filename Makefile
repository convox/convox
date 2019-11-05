.PHONY: all build clean clean-package compress generate generate-k8s generate-provider mocks package release test

commands = api atom build router

binaries = $(addprefix $(GOPATH)/bin/, $(commands))
sources  = $(shell find . -name '*.go')
version := dev-$(shell date +%Y%m%d%H%M%S)

all: build

build: $(binaries)

clean: clean-package

clean-package:
	find . -name '*-packr.go' -delete

compress: $(binaries)
	upx-ucl -1 $^

generate: generate-provider generate-k8s

generate-k8s:
	make -C pkg/atom generate
	make -C provider/k8s generate

generate-provider:
	go run cmd/generate/main.go controllers > pkg/api/controllers.go
	go run cmd/generate/main.go routes > pkg/api/routes.go
	go run cmd/generate/main.go sdk > sdk/methods.go

mocks: generate-provider
	make -C pkg/atom mocks
	make -C pkg/structs mocks
	mockery -case underscore -dir pkg/start -outpkg sdk -output pkg/mock/start -name Interface
	mockery -case underscore -dir sdk -outpkg sdk -output pkg/mock/sdk -name Interface
	mockery -case underscore -dir vendor/github.com/convox/stdcli -outpkg stdcli -output pkg/mock/stdcli -name Executor

package:
	$(GOPATH)/bin/packr

release:
	docker build -t convox/convox:$(version) .
	docker push convox/convox:$(version)

test:
	env PROVIDER=test go test -covermode atomic -coverprofile coverage.txt ./...

$(binaries): $(GOPATH)/bin/%: $(sources)
	go install -mod=vendor --ldflags="-s -w" ./cmd/$*
