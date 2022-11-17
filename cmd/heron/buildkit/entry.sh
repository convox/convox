#!/bin/bash

buildctl \
    --addr tcp://buildkitd:1234 \
    build --frontend dockerfile.v0 --local context=/nodejs --local dockerfile=/nodejs \
    -output type=image,name=az2bp1nmg9mes7r.azurecr.io/heron-test:nodejs,push=true






# -output type=image,name=470778123668.dkr.ecr.us-east-1.amazonaws.com/heron-test:nodejs,push=true