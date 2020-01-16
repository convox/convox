# build

## build

Create a build

### Usage

    convox build [dir]

### Examples

    $ convox build --no-cache --description "My latest build" 
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating https://index.docker.io/v1/: Login Succeeded
    Authenticating 316421501735.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    ...
    ...
    Running: docker tag convox/mynewapp:web.BVBZUSTXALE 316421501735.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BVBZUSTXALE
    Running: docker push 316421501735.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BVBZUSTXALE
    Build:   BVBZUSTXALE
    Release: RZGMCKQOATO