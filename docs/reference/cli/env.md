---
title: "env"
slug: env
url: /reference/cli/env
---
# env

## env

List env vars

### Usage
```bash
    convox env
```
### Examples
```bash
    $ convox env
    COUNT=0
    FOO=bar
    TEST=dummy
```
## env edit

Edit env interactively

### Usage
```bash
    convox env edit
```
### Examples
```bash
    $ convox env edit
    Setting ... OK
    Release: RABCDEFGHI
```
## env get

Get an env var

### Usage
```bash
    convox env get <var>
```
### Examples
```bash
    $ convox env get FOO
    bar
```
## env set

Set env var(s)

### Usage
```bash
    convox env set <key=value> [key=value]...
```
### Examples
```bash
    $ convox env set FOO=bar
    Setting FOO... OK
    Release: RABCDEFGHI
```
## env unset

Unset env var(s)

### Usage
```bash
    convox env unset <key> [key]...
```
### Examples
```bash
    $ convox env unset FOO
    Unsetting FOO... OK
    Release: RABCDEFGHI
```