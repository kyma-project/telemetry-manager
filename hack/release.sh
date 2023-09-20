#!/usr/bin/env bash

readonly MODULE_REGISTRY="europe-docker.pkg.dev/kyma-project/prod/unsigned"
readonly GCP_ACCESS_TOKEN=$(gcloud auth application-default print-access-token)

function create_module() {
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    ${KUSTOMIZE} build config/default > telemetry-manager.yaml
    git remote add origin https://github.com/kyma-project/telemetry-manager
    ${KYMA} alpha create module --module-config-file=module_config.yaml --registry ${MODULE_REGISTRY} -c oauth2accesstoken:${GCP_ACCESS_TOKEN} -o moduletemplate.yaml --ci
}

function create_github_release() {
    # rename the file for Telemetry default CR to have a better naming as a release artefact
    cp ./config/samples/operator_v1alpha1_telemetry.yaml telemetry-default-cr.yaml
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
