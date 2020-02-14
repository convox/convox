modvendor
=========

Simple tool to copy additional module files into a local ./vendor folder. This
tool should be run after `go mod vendor`.

`go get -u github.com/goware/modvendor`

## Usage

```
$ GO111MODULE=on go mod vendor
$ modvendor -copy="**/*.c **/*.h **/*.proto" -v
```

## LICENSE

MIT
