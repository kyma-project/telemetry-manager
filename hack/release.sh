#!/usr/bin/env bash

readonly MODULE_REGISTRY="europe-docker.pkg.dev/kyma-project/prod/unsigned"
readonly GCP_ACCESS_TOKEN=$(gcloud auth application-default print-access-token)

function create_module() {
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    ${KUSTOMIZE} build config/default > manifests.yaml
    ${KYMA} alpha create module --module-config-file=module_config.yaml --registry ${MODULE_REGISTRY} -c oauth2accesstoken:${GCP_ACCESS_TOKEN} --ci
}

function create_github_release() {
    git remote add origin git@github.com:kyma-project/telemetry-manager.git
    git reset --hard
    curl -sL https://git.io/goreleaser | VERSION=${GORELEASER_VERSION} bash
}

function main() {
    # Create the module and push its image to the prod registry defined in MODULE_REGISTRY
    create_module

    # Create github release entry using goreleaser
    create_github_release
}

main
