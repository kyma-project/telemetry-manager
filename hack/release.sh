#!/usr/bin/env bash
source .env

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

readonly LOCALBIN=${LOCALBIN:-$(pwd)/bin}
readonly KUSTOMIZE=${KUSTOMIZE:-$LOCALBIN/kustomize}
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

get_previous_release_version() {
    TAG_LIST=($(git tag --sort=-creatordate | egrep "^[0-9]+.[0-9]+.[0-9]$"))
    if [[ "${TAG_LIST[0]}" =~ ^[0-9]+.[0-9]+.[2-9]$ ]]
    then
          # get the list of tags in a reverse chronological order including patch tags
          TAG_LIST_WITH_PATCH=($(git tag --sort=-creatordate | egrep "^[0-9]+.[0-9]+.[1-9]$"))
          export GORELEASER_PREVIOUS_TAG=${TAG_LIST_WITH_PATCH[1]}
    else
          # get the list of tags in a reverse chronological order excluding patch tags
          TAG_LIST_WITHOUT_PATCH=($(git tag --sort=-creatordate | egrep "^[0-9]+.[0-9]+.[0-9]$"))
          export GORELEASER_PREVIOUS_TAG=${TAG_LIST_WITHOUT_PATCH[1]}
    fi
}

get_new_release_version() {
    # get the list of tags in a reverse chronological order
    TAG_LIST=($(git tag --sort=-creatordate))
    export GORELEASER_CURRENT_TAG=${TAG_LIST[0]}
}

function create_github_release() {
    echo "Creating the Github release"
    git reset --hard
    curl -sL https://git.io/goreleaser | VERSION=${GORELEASER_VERSION} bash
}

function main() {
    prepare_release_artefacts
    get_new_release_version
    get_previous_release_version
    create_github_release
}

main
