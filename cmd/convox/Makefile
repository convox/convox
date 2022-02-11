.PHONY: all build clean release

gopath := $(shell mktemp -d)
work   := $(gopath)/src/github.com/convox/convox

all: build

build:
	mkdir pkg/
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-linux-amd64
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-linux-arm64
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-darwin-amd64
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.image=$(IMAGE)" -o pkg/convox-darwin-arm64

clean:
	rm -f pkg/convox-*
