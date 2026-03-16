---
title: "Command Line Interface"
slug: command-line-interface
url: /installation/cli
---
# Command Line Interface

## Linux

### x86_64 / amd64

```bash
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```

### arm64

```bash
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux-arm64 -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```

## macOS

### x86_64 / amd64

```bash
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```

### arm64

```bash
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos-arm64 -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```

## See Also

- [Development Rack](/installation/development-rack) for setting up a local development environment
- [Production Rack](/installation/production-rack) for deploying to cloud providers
