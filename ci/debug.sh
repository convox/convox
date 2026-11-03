#!/bin/bash

export KUBECONFIG=~/.kube/config.${PROVIDER}.${RACK_NAME}

# terraform writes this file on gcp and do only; aws reads eks directly so a broken rack api cannot hide the debug output
if [ ! -s "$KUBECONFIG" ]; then
  mkdir -p ~/.kube
  if [ "${PROVIDER}" = "aws" ]; then
    region=$(echo " ${RACK_PARAMS:-} " | sed -nE 's/.* region=([^ ]+).*/\1/p')
    aws eks update-kubeconfig --name ${RACK_NAME} --region ${region:-$REGION} --kubeconfig "$KUBECONFIG" >/dev/null 2>&1 || true
  else
    convox rack kubeconfig -r ${RACK_NAME} > "$KUBECONFIG" 2>/dev/null || true
    kubectl config set-cluster "$(kubectl config view -o jsonpath='{.clusters[0].name}')" --insecure-skip-tls-verify=true >/dev/null 2>&1 || true
  fi
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
