---
title: "builds"
draft: false
slug: builds
url: /reference/cli/builds
---
# builds

## builds

List builds

### Usage
```html
    convox builds
```
### Examples
```html
    $ convox builds
    ID           STATUS    RELEASE      STARTED       ELAPSED  DESCRIPTION
    BABCDEFGHIJ  complete  RABCDEFGHIJ  1 week ago    17s
    BBCDEFGHIJK  complete  RBCDEFGHIJK  1 week ago    9s       My latest build
    BCDEFGHIJKL  failed                 1 week ago    3s       My latest build
```
## builds export

Export a build

### Usage
```html
    convox builds export <build>
```
### Examples
```html
    $ convox builds export BABCDEFGHIJ --file build.tgz
    Exporting build... OK
```
## builds import

Import a build

### Usage
```html
    convox builds import
```
### Examples
```html
    $ convox builds import --file output.tgz
    Importing build... OK, RFGHIJKLMNOP
```
## builds info

Get information about a build

### Usage
```html
    convox builds info <build>
```
### Examples
```html
    $ convox builds info BABCDEFGHIJ
    Id           BABCDEFGHIJ
    Status       complete
    Release      RABCDEFGHIJ
    Description  My latest build
    Started      1 week ago
    Elapsed      17s
```
## builds logs

Get logs for a build

### Usage
```html
    convox builds logs <build>
```
### Examples
```html
    $ convox builds logs BABCDEFGHIJ
    Authenticating https://index.docker.io/v1/: Login Succeeded
    Authenticating 1234567890.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    ...
    ...
    Running: docker tag convox/myapp:web.BABCDEFGHI 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Running: docker push 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
```