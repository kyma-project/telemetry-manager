#!/usr/bin/env sh

bin/k3d registry create kyma-registry --port 5001
bin/k3d cluster create kyma --registry-use kyma-registry:5001 --image rancher/k3s:v$K8S_VERSION-k3s1 --api-port 6550

kubectl create ns kyma-system
