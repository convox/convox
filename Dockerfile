## development #################################################################

FROM golang:1.25 AS development

ARG DOCKER_ARCH=x86_64
ARG KUBECTL_ARCH=amd64


RUN apt-get update && apt-get -y install default-mysql-client postgresql-client redis-tools telnet

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN curl -s https://download.docker.com/linux/static/stable/$DOCKER_ARCH/docker-20.10.7.tgz | \
  tar -C /usr/bin --strip-components 1 -xz

RUN curl -Ls https://dl.k8s.io/release/v1.33.5/bin/linux/$KUBECTL_ARCH/kubectl -o /usr/bin/kubectl && \
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

FROM golang:1.25 AS package

WORKDIR /usr/src/convox

RUN apt-get update && apt-get install -y libudev-dev

COPY . .

RUN make package build compress

## production ##################################################################

FROM ubuntu:24.04

ARG DOCKER_ARCH=x86_64
ARG KUBECTL_ARCH=amd64

RUN apt-get -qq update && apt-get -qq -y install curl default-mysql-client postgresql-client redis-tools telnet skopeo

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN curl -s https://download.docker.com/linux/static/stable/$DOCKER_ARCH/docker-20.10.7.tgz | \
  tar -C /usr/bin --strip-components 1 -xz

RUN curl -Ls https://dl.k8s.io/release/v1.33.5/bin/linux/$KUBECTL_ARCH/kubectl -o /usr/bin/kubectl && \
  chmod +x /usr/bin/kubectl

ENV DEVELOPMENT=false
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

WORKDIR /app

COPY --from=package /go/bin/api      $GOPATH/bin/
COPY --from=package /go/bin/atom     $GOPATH/bin/
COPY --from=package /go/bin/build    $GOPATH/bin/
COPY --from=package /go/bin/convox   $GOPATH/bin/
COPY --from=package /go/bin/docs     $GOPATH/bin/
COPY --from=package /go/bin/resolver $GOPATH/bin/

COPY --from=package /usr/src/convox/bin/docs bin/docs
