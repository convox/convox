---
title: "resources"
slug: resources
url: /reference/cli/resources
---
# resources

## resources

List resources

### Usage
```bash
    convox resources
```
### Examples
```bash
    $ convox resources
    NAME      TYPE      URL
    database  postgres  postgres://app:123456-7890-abcd-ef01-234567890abc@test-nodejs-resourcedatabase-abcd.efgh.us-east-1.rds.amazonaws.com:5432/app
```
## resources console

Start a console for a resource

### Usage
```bash
    convox resources console <resource>
```
### Examples
```bash
    $ convox resources console database
```
## resources export

Export data from a resource

### Usage
```bash
    convox resources export <resource>
```
### Examples
```bash
    $ convox resources export database
```
## resources import

Import data to a resource

### Usage
```bash
    convox resources import <resource>
```
### Examples
```bash
    $ convox resources import database --file dump.tgz
```

## resources info

Get information about a resource

### Usage
```bash
    convox resources info <resource>
```
### Examples
```bash
    $ convox resources info database
    Name  database
    Type  postgres
    URL   postgres://app:123456-7890-abcd-ef01-234567890abc@test-nodejs-resourcedatabase-abcd.efgh.us-east-1.rds.amazonaws.com:5432/app
```
## resources proxy

Proxy a local port to a resource

### Usage
```bash
    convox resources proxy <resource>
```
### Examples
```bash
    $ convox resources proxy database
    proxying localhost:5432 to test-nodejs-resourcedatabase-abcd.efgh.us-east-1.rds.amazonaws.com:5432

    $ convox resources proxy database --port 65432
    proxying localhost:65432 to test-nodejs-resourcedatabase-abcd.efgh.us-east-1.rds.amazonaws.com:5432
```
## resources url

Get url for a resource

### Usage
```bash
    convox resources url <resource>
```
### Examples
```bash
    $ convox resources url database
    postgres://app:123456-7890-abcd-ef01-234567890abc@test-nodejs-resourcedatabase-abcd.efgh.us-east-1.rds.amazonaws.com:5432/app
```