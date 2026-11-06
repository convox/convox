---
title: "Command Line Interface"
description: "Install the Convox CLI on Linux or macOS (amd64 or arm64) by downloading the release binary, then verify it with convox version."
slug: command-line-interface
url: /installation/cli
---
# Command Line Interface

## Linux

### Linux x86_64 / amd64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

### Linux arm64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux-arm64 -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

## macOS

### macOS x86_64 / amd64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

### macOS arm64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos-arm64 -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

## Verify the Install

After moving the binary into place, confirm the CLI is installed and on your `PATH`:

```bash
$ convox version
```

A successful install prints the client version. If you see a "command not found" error, make sure `/usr/local/bin` is on your `PATH` and that the binary is executable.

## See Also

- [Development Rack](/installation/development-rack) for setting up a local development environment
- [Production Rack](/installation/production-rack) for deploying to cloud providers
