#!/usr/bin/env bash

readonly GCP_ACCESS_TOKEN=$(gcloud auth application-default print-access-token)

function create_module() {
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    ${KYMA} alpha create module --name kyma-project.io/module/${MODULE_NAME} --version ${MODULE_VERSION} --channel ${MODULE_CHANNEL} --default-cr ${MODULE_CR_PATH} --registry ${MODULE_REGISTRY} -c oauth2accesstoken:${GCP_ACCESS_TOKEN} --ci
}

function create_github_release() {
    # Create github release entry using goreleaser
    git remote add origin git@github.com:kyma-project/telemetry-manager.git
	git reset --hard
	curl -sL https://git.io/goreleaser | VERSION=${GORELEASER_VERSION} bash
}

function main() {
    create_module
    create_github_release
}

main
