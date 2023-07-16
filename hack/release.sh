#!/usr/bin/env bash

readonly MODULE_REGISTRY="europe-docker.pkg.dev/kyma-project/prod/unsigned"
readonly GCP_ACCESS_TOKEN=$(gcloud auth application-default print-access-token)

function create_module() {
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    # create module command uses Kustomization files defined in "config/default"
    ${KYMA} alpha create module --name kyma-project.io/module/${MODULE_NAME} --version ${MODULE_VERSION} --channel ${MODULE_CHANNEL} --default-cr ${MODULE_CR_PATH} --registry ${MODULE_REGISTRY} -c oauth2accesstoken:${GCP_ACCESS_TOKEN} --ci
}

function apply_doc_url_annotation() {
    kubectl annotate --local=true -f template.yaml operator.kyma-project.io/doc-url=https://github.com/kyma-project/telemetry-manager/tree/${RELEASE_TAG}/docs/user -o yaml > temporary-template.yaml
    mv temporary-template.yaml template.yaml
}

function create_github_release() {
    git remote add origin git@github.com:kyma-project/telemetry-manager.git
	git reset --hard
	curl -sL https://git.io/goreleaser | VERSION=${GORELEASER_VERSION} bash
}

function main() {
    # Create the module and push its image to the prod registry defined in MODULE_REGISTRY
    create_module

    # Apply doc-url annotation
    apply_doc_url_annotation

    # Create github release entry using goreleaser
    create_github_release
}

main
