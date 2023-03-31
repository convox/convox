.PHONY: all build clean release

gopath := $(shell mktemp -d)
work   := $(gopath)/src/github.com/convox/convox

all: build

build:
	mkdir pkg/
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags=hidraw -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-linux-amd64
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags=hidraw -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-linux-arm64
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-darwin-arm64

	go env -w GO111MODULE=auto
	tar cz -C ../.. . | docker run -v $(gopath)/src/github.com/convox/convox:/convox -i ubuntu tar xz -C /convox
	mkdir -p $(work) && cd $(work) && env GOPATH=$(gopath) $(shell go env GOPATH)/bin/xgo -go 1.17.x -pkg cmd/convox -out cmd/convox/pkg/convox -targets 'darwin/amd64' -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" .
	mkdir -p pkg && docker run -v $(gopath):/gopath -i ubuntu tar czv /gopath/src/github.com/convox/convox/cmd/convox/pkg | tar xzv -C pkg --strip-components 8

clean:
	rm -f pkg/convox-*
