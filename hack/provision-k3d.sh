#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

bin/k3d registry create kyma-registry --port 5001
bin/k3d cluster create kyma --registry-use kyma-registry:5001 --image rancher/k3s:v$K8S_VERSION-k3s1 --api-port 6550

kubectl create ns kyma-system
