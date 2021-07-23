.PHONY: all build clean release

gopath := $(shell mktemp -d)
work   := $(gopath)/src/github.com/convox/convox/cmd/convox

all: build

build:
	go env -w GO111MODULE=auto
	tar cz -C ../.. . | docker run -v $(gopath)/src/github.com/convox/convox:/convox -i ubuntu tar xz -C /convox
	mkdir -p $(work) && cd $(work) && env GOPATH=$(gopath) $(shell go env GOPATH)/bin/xgo -out pkg/convox -targets 'darwin/amd64,darwin/arm64,linux/amd64,linux/arm64' -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" .
	mkdir -p pkg && docker run -v $(gopath):/gopath -i ubuntu tar czv /gopath/src/github.com/convox/convox/cmd/convox/pkg | tar xzv -C pkg --strip-components 8

clean:
	rm -f pkg/convox-*