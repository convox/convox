---
title: "build"
draft: false
slug: build
url: /reference/cli/build
---
# build

## build

Create a build

### Usage
```html
    convox build [dir]
```
### Examples
```html
    $ convox build --no-cache --description "My latest build" 
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating https://index.docker.io/v1/: Login Succeeded
    Authenticating 1234567890.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    ...
    ...
    Running: docker tag convox/myapp:web.BABCDEFGHI 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Running: docker push 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Build:   BABCDEFGHI
    Release: RABCDEFGHI
```