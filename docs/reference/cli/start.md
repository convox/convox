# start

## start

Start an application for local development

### Usage

    convox start [service] [service...] [options]

### Options

    -m <file.yml> allows to specify an alternative manifest file (convox.yml by default)

### Examples

    $ convox start
    build  | uploading source
    build  | starting build
    build  | Authenticating registry.convox/nodejs: Login Succeeded
    build  | Building: .
    build  | Sending build context to Docker daemon  171.5kB
    build  | Step 1/5 : FROM node:10.16.3-alpine
    build  |  ---> b95baba1cfdb
    build  | Step 2/5 : WORKDIR /usr/src/app
    build  |  ---> Using cache
    build  |  ---> 8b06151e1836
    build  | Step 3/5 : COPY . /usr/src/app
    build  |  ---> Using cache
    build  |  ---> 02881c46489e
    build  | Step 4/5 : EXPOSE 3000
    build  |  ---> Using cache
    build  |  ---> 79271f2d681d
    build  | Step 5/5 : CMD ["node", "app.js"]
    build  |  ---> Using cache
    build  |  ---> fd45ec629418
    build  | Successfully built fd45ec629418
    build  | Successfully tagged c9b4c3c01a44a3037f2b6e57b9269de8382e772b:latest
    build  | Running: docker tag c9b4c3c01a44a3037f2b6e57b9269de8382e772b convox/nodejs:web.BABCDEFGHI
    build  | Running: docker tag convox/nodejs:web.BABCDEFGHI registry.convox/nodejs:web.BABCDEFGHI
    build  | Running: docker push registry.convox/nodejs:web.BABCDEFGHI
    convox | starting sync from . to /usr/src/app on web
    web    | Scaled up replica set web-58d8446884 to 1
    web    | Created pod: web-58d8446884-gfkxz
    web    | Successfully assigned convox-nodejs/web-58d8446884-gfkxz to docker-desktop
    web    | Container image "registry.convox/nodejs:web.BABCDEFGHI" already present on machine
    web    | Created container main
    web    | Server running at http://0.0.0.0:3000/
    web    | Started container main


