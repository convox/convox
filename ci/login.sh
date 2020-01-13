#!/bin/bash

cd install/${PROVIDER}

RACK_URL=$(terraform output rack_url)

echo "::add-mask::${RACK_URL}"
echo "::set-env name=RACK_URL::${RACK_URL}"