#!/bin/bash

export KUBECONFIG=~/.kube/config.${PROVIDER}.${RACK_NAME}

set -x

kubectl get node
kubectl describe node
kubectl get all -n ${RACK_NAME}-system || true
kubectl logs deployment/api -n ${RACK_NAME}-system || true
kubectl logs deployment/atom -n ${RACK_NAME}-system || true
kubectl logs deployment/registry -n ${RACK_NAME}-system || true
kubectl logs deployment/resolver -n ${RACK_NAME}-system || true
kubectl logs deployment/router -n ${RACK_NAME}-system || true
kubectl get event -n ${RACK_NAME}-system || true
kubectl get all -n ${RACK_NAME}-httpd || true
kubectl logs deployment/web -n ${RACK_NAME}-httpd || true
kubectl get event -n ${RACK_NAME}-httpd || true
