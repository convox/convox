# env

## env

List env vars

### Usage

    convox env

### Examples

    $ convox env
    COUNT=0
    FOO=bar
    TEST=dummy

## env edit

Edit env interactively

### Usage

    convox env edit

### Examples

    $ convox env edit
    Setting ... OK
    Release: RABCDEFGHI

## env get

Get an env var

### Usage

    convox env get <var>

### Examples

    $ convox env get FOO
    bar

## env set

Set env var(s)

### Usage

    convox env set <key=value> [key=value]...

### Examples

    $ convox env set FOO=bar
    Setting FOO... OK
    Release: RABCDEFGHI

## env unset

Unset env var(s)

### Usage

    convox env unset <key> [key]...

### Examples

    $ convox env unset FOO
    Unsetting FOO... OK
    Release: RABCDEFGHI