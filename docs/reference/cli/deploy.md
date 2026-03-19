---
title: "deploy"
slug: deploy
url: /reference/cli/deploy
---
# deploy

## deploy

Create and promote a build

### Usage
```bash
    convox deploy [dir]
```
### Examples
```bash
    $ convox deploy
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
    Promoting RABCDEFGHI...
    ...
    ...
    2026-03-18T15:41:16Z system/cloudformation aws/cfm test-myapp-ServiceApp-ZNV5T8E1R2XQ DELETE_COMPLETE ExecutionRole
    2026-03-18T15:41:27Z system/cloudformation aws/cfm test-myapp DELETE_COMPLETE ServiceApp
    2026-03-18T15:41:27Z system/cloudformation aws/cfm test-myapp UPDATE_COMPLETE test-myapp
    OK
```

### Pass build time env vars

You can pass env vars that will only exist at build time.

```bash
    $ convox deploy --build-args "BUILD_ENV1=val1" --build-args "BUILD_ENV2=val2"
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
    Promoting RABCDEFGHI...
    ...
    ...
    2026-03-18T15:41:16Z system/cloudformation aws/cfm test-myapp-ServiceApp-ZNV5T8E1R2XQ DELETE_COMPLETE ExecutionRole
    2026-03-18T15:41:27Z system/cloudformation aws/cfm test-myapp DELETE_COMPLETE ServiceApp
    2026-03-18T15:41:27Z system/cloudformation aws/cfm test-myapp UPDATE_COMPLETE test-myapp
    OK
```

## See Also

- [Deploying Changes](/deployment/deploying-changes) for deployment workflow
- [Rolling Updates](/deployment/rolling-updates) for deployment strategies
