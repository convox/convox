.PHONY: all build clean clean-package compress generate mocks package release test

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

generate:
	go run cmd/generate/main.go controllers > pkg/api/controllers.go
	go run cmd/generate/main.go routes > pkg/api/routes.go
	go run cmd/generate/main.go sdk > sdk/methods.go
	make -C pkg/atom generate
	make -C provider/k8s generate

mocks: generate
	make -C pkg/atom mocks
	make -C pkg/structs mocks

package:
	$(GOPATH)/bin/packr

release:
	docker build -t convox/convox:$(version) .
	docker push convox/convox:$(version)

test:
	env PROVIDER=test go test -covermode atomic -coverprofile coverage.txt ./...

$(binaries): $(GOPATH)/bin/%: $(sources)
	go install -mod=vendor --ldflags="-s -w" ./cmd/$*
