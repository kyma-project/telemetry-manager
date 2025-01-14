#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

CURRENT_COMMIT=$(git rev-parse --abbrev-ref HEAD)
LATEST_TAG=$(git tag --sort=-creatordate | sed -n 1p)

echo "switch git revision to last release"
git restore .
git checkout $LATEST_TAG

echo "build manager image for version $LATEST_TAG"
# replace with ./hack/build-image.sh after merge
IMG=localhost:5001/telemetry-manager:latest
export IMG
make docker-build
make docker-push

echo "deploy manager image for version $LATEST_TAG"
IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev

echo "rollback to current git ref already to have make target and script changes available"
git restore .
git checkout $CURRENT_COMMIT

echo "setup ginkgo"
make ginkgo

echo "run upgrade test"
bin/ginkgo run --tags e2e --flake-attempts=5 --label-filter="operational" -v test/e2e

echo "wait for namespace termination"
hack/wait-for-namespaces.sh

echo "build manager image for version $CURRENT_COMMIT"
hack/build-image.sh

echo "deploy manager image for version $CURRENT_COMMIT"
IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev

echo "run upgrade test"
bin/ginkgo run --tags e2e --flake-attempts=5 --label-filter="operational" -v test/e2e
