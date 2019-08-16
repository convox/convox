.PHONY: all build clean clean-package compress dev-aws package release test

commands = atom router

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

dev-aws: release
	echo "release = \"$(version)\"" > terraform/rack/aws/terraform.tfvars
	cd terraform/rack/aws && terraform apply

package:
	$(GOPATH)/bin/packr

release:
	docker build -t convox/convox:$(version) .
	docker push convox/convox:$(version)

test:
	env PROVIDER=test go test -covermode atomic -coverprofile coverage.txt ./...

$(binaries): $(GOPATH)/bin/%: $(sources)
	go install -mod=vendor --ldflags="-s -w" ./cmd/$*
