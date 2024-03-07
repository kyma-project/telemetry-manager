#!/usr/bin/env bash
source .env

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

readonly LOCALBIN=${LOCALBIN:-$(pwd)/bin}
readonly KUSTOMIZE=${KUSTOMIZE:-$(LOCALBIN)/kustomize}
readonly GORELEASER_VERSION="${GORELEASER_VERSION:-$ENV_GORELEASER_VERSION}"
readonly IMG="${IMG:-$ENV_IMG}"

function prepare_release_artefacts() {
     echo "Preparing release artefacts"
     cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
     # Create the resources file that is used for creating the ModuleTemplate for fast and regular channels
     ${KUSTOMIZE} build config/default > telemetry-manager.yaml
     # Create the resources file that is used for creating the ModuleTemplate for experimental channel
     ${KUSTOMIZE} build config/development > telemetry-manager-dev.yaml
     # Rename the file for Telemetry default CR to have a better naming as a release artefact
     cp ./config/samples/operator_v1alpha1_telemetry.yaml telemetry-default-cr.yaml
}

function create_github_release() {
    echo "Creating the Github release"
    git reset --hard
    curl -sL https://git.io/goreleaser | VERSION=${GORELEASER_VERSION} bash
}

function main() {
    prepare_release_artefacts
    create_github_release
}

main
