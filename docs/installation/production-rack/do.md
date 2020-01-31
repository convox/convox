# Digital Ocean

## Initial Setup

### Convox CLI

- [Install the Convox CLI](../cli.md)

## Environment

The following environment variables are required:

- `DIGITALOCEAN_ACCESS_ID`
- `DIGITALOCEAN_SECRET_KEY`
- `DIGITALOCEAN_TOKEN`

### Create Token

Go to https://cloud.digitalocean.com/account/api/tokens and generate a new Personal Access Token.

- `DIGITALOCEAN_TOKEN` is the token you just created
  
### Create Spaces Access Key

Go to https://cloud.digitalocean.com/account/api/tokens and generate a new Spaces Access Key.

- `DIGITALOCEAN_ACCESS_ID` is the resulting Key
- `DIGITALOCEAN_SECRET_KEY` is the Secret

## Install Rack

    $ convox rack install do <name>