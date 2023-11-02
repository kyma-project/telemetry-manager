#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

echo "switch git revision to last release"
LATEST_TAG=$(git tag --sort=-creatordate | sed -n 1p)
git restore .
git checkout $LATEST_TAG

echo "build manager image for version $LATEST_TAG"
./build-image.sh

echo "deploy manager image for version $LATEST_TAG"
IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev

echo "rollback to current git ref already to have make target and script changes available"
CURRENT_COMMIT=$(git rev-parse --abbrev-ref HEAD)
git restore .
git checkout $CURRENT_COMMIT

echo "run upgrade test"
make run-upgrade-test

echo "wait for namespace termination"
./wait-for-namespaces.sh

echo "build manager image for version $CURRENT_COMMIT"
./build-image.sh

echo "deploy manager image for version $CURRENT_COMMIT"
IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev

echo "run upgrade test"
make run-upgrade-test
