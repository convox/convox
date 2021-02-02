# run

## run

Execute a command in a new process

### Usage

    convox run <service> <command>

### Examples

    $ convox run web sh
    /usr/src/app #

Run against a specific release:

    $ convox run --release RABCDEFGHIJ web sh
    /usr/src/app #
