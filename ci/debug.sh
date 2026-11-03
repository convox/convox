#!/bin/bash

export KUBECONFIG=~/.kube/config.${PROVIDER}.${RACK_NAME}

# terraform writes this file on gcp and do only
if [ ! -s "$KUBECONFIG" ]; then
  mkdir -p ~/.kube
  convox rack kubeconfig -r ${RACK_NAME} > "$KUBECONFIG" 2>/dev/null || true
fi

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
kubectl get svc -n ${RACK_NAME}-system -o wide || true
kubectl get event -A --field-selector involvedObject.kind=Service || true
kubectl get all -n ${RACK_NAME}-httpd || true
kubectl logs deployment/web -n ${RACK_NAME}-httpd || true
kubectl get event -n ${RACK_NAME}-httpd || true
