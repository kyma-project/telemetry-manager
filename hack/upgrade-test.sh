#!/usr/bin/env bash

TEST_INCEPTION_TAG="0.5.0" # The tag which predates the introduction of the upgrade test.
CURRENT_COMMIT=$(git rev-parse --abbrev-ref HEAD)
TAG_LIST=$(git tag --sort=-creatordate)
LATEST_TAG=${TAG_LIST[0]}

test_at_revision()
{
    GIT_COMMIT="$1"
    DOCKER_TAG="$2"

    git restore .
    git checkout $GIT_COMMIT

    IMG=localhost:5001/telemetry-manager:$DOCKER_TAG
    export IMG

    make docker-build
    make docker-push
    IMG=k3d-kyma-registry:5000/telemetry-manager:$DOCKER_TAG make deploy-release

    ./bin/ginkgo run --tags e2e --flake-attempts=5 --label-filter="operational && !metrics" -v ./test/e2e
}

bin/k3d registry create kyma-registry --port 5001
bin/k3d cluster create kyma --registry-use kyma-registry:5001 --image rancher/k3s:v$K8S_VERSION-k3s1 --api-port 6550
kubectl create ns kyma-system

# Run test suite for the latest released version if it is after the v0.5.0.
if [ "$LATEST_TAG" != "$TEST_INCEPTION_TAG" ];
then
    echo -e "Running the test suite on the latest released version\\n"
    if ! test_at_revision $LATEST_TAG latest;
    then
        git restore .
        git checkout $CURRENT_COMMIT
        exit 1
    fi
fi

echo -e "Running the test suite on the recent version\\n"
test_at_revision $CURRENT_COMMIT recent
