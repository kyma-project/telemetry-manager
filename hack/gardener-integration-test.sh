#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

GIT_COMMIT_SHA=$(git rev-parse --short=8 HEAD)
GIT_COMMIT_DATE=$(git show -s --format=%cd --date=format:'v%Y%m%d' ${GIT_COMMIT_SHA})
IMG="europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:${GIT_COMMIT_DATE}-${GIT_COMMIT_SHA} make deploy-dev"

function run-tests-with-git-image () {
    kubectl create namespace kyma-system
    hack/run-tests.sh integration istio
}

function main() {
    make provision-gardener
    run-tests-with-git-image
    make deprovision-gardener || (make deprovision-gardener && false) # TODO: Why do we need this?
}

main