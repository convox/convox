## development #################################################################

FROM golang:1.13 AS development

RUN apt-get update && apt-get -y install default-mysql-client postgresql-client redis-tools telnet

RUN curl -s https://download.docker.com/linux/static/stable/x86_64/docker-18.03.1-ce.tgz | \
  tar -C /usr/bin --strip-components 1 -xz

RUN curl -Ls https://storage.googleapis.com/kubernetes-release/release/v1.17.3/bin/linux/amd64/kubectl -o /usr/bin/kubectl && \
  chmod +x /usr/bin/kubectl

RUN curl -Ls https://github.com/mattgreen/watchexec/releases/download/1.8.6/watchexec-1.8.6-x86_64-unknown-linux-gnu.tar.gz | \
  tar -C /usr/bin --strip-components 1 -xz

ENV DEVELOPMENT=true

WORKDIR /usr/src/convox

COPY go.mod go.sum ./
COPY vendor vendor

RUN go build -mod=vendor --ldflags="-s -w" $(go list -mod=vendor ./vendor/...)

COPY . .

RUN make build

## package #####################################################################

FROM golang:1.13 AS package

RUN apt-get update && apt-get -y install upx-ucl

WORKDIR /usr/src/convox

COPY --from=development /usr/src/convox .

RUN make package build compress

## production ##################################################################

FROM ubuntu:18.04

RUN apt-get -qq update && apt-get -qq -y install curl default-mysql-client postgresql-client redis-tools telnet

RUN curl -s https://download.docker.com/linux/static/stable/x86_64/docker-18.03.1-ce.tgz | \
  tar -C /usr/bin --strip-components 1 -xz

RUN curl -Ls https://storage.googleapis.com/kubernetes-release/release/v1.17.2/bin/linux/amd64/kubectl -o /usr/bin/kubectl && \
  chmod +x /usr/bin/kubectl

ENV DEVELOPMENT=false
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

WORKDIR /app

COPY --from=package /go/bin/api      $GOPATH/bin/
COPY --from=package /go/bin/atom     $GOPATH/bin/
COPY --from=package /go/bin/build    $GOPATH/bin/
COPY --from=package /go/bin/docs     $GOPATH/bin/
COPY --from=package /go/bin/resolver $GOPATH/bin/
COPY --from=package /go/bin/router   $GOPATH/bin/

COPY --from=package /usr/src/convox/bin/docs bin/docs
