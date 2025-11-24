FROM golang:1.24.11

RUN curl -s https://download.docker.com/linux/static/stable/x86_64/docker-18.03.1-ce.tgz | \
    tar -C /usr/bin --strip-components 1 -xz

COPY . $GOPATH/src/github.com/convox/convox

RUN go install github.com/convox/convox/cmd/build
