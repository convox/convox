---
title: "env"
draft: false
slug: env
url: /reference/cli/env
---
# env

## env

List env vars

### Usage
```html
    convox env
```
### Examples
```html
    $ convox env
    COUNT=0
    FOO=bar
    TEST=dummy
```
## env edit

Edit env interactively

### Usage
```html
    convox env edit
```
### Examples
```html
    $ convox env edit
    Setting ... OK
    Release: RABCDEFGHI
```
## env get

Get an env var

### Usage
```html
    convox env get <var>
```
### Examples
```html
    $ convox env get FOO
    bar
```
## env set

Set env var(s)

### Usage
```html
    convox env set <key=value> [key=value]...
```
### Examples
```html
    $ convox env set FOO=bar
    Setting FOO... OK
    Release: RABCDEFGHI
```
## env unset

Unset env var(s)

### Usage
```html
    convox env unset <key> [key]...
```
### Examples
```html
    $ convox env unset FOO
    Unsetting FOO... OK
    Release: RABCDEFGHI
```