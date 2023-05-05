#!/usr/bin/env bash

readonly MODULE_VERSION="${MODULE_VERSION:-0.0.1}"
readonly MODULE_CHANNEL="${MODULE_CHANNEL:-fast}"
readonly REGISTRY_PORT="${REGISTRY_PORT:-5001}" 
readonly CLUSTER_NAME="${CLUSTER_NAME:-kyma}" 
readonly MODULE_NAME="${MODULE_NAME:-telemetry}"
readonly REGISTRY_NAME="${REGISTRY_NAME:-${CLUSTER_NAME}-registry}"
readonly MODULE_REGISTRY="${MODULE_REGISTRY:-localhost:${REGISTRY_PORT}}"

function main() {    
    # Create a k3d cluster using Kyma cli
    ${KYMA} provision k3d --registry-port ${REGISTRY_PORT} --name ${CLUSTER_NAME} --ci
    
    # Build and push manager image to a local k3d registry
    export IMG=localhost:${REGISTRY_PORT}/${MODULE_NAME}-manager
    make docker-build
    make docker-push
    
    # Build the module and push it to a local k3d registry
    export IMG=k3d-${REGISTRY_NAME}:${REGISTRY_PORT}/${MODULE_NAME}-manager
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    ${KYMA} alpha create module --name kyma-project.io/module/${MODULE_NAME} --version ${MODULE_VERSION} --channel ${MODULE_CHANNEL} --default-cr ./config/samples/operator_v1alpha1_telemetry.yaml --registry ${MODULE_REGISTRY} --insecure --ci

    # Create template-k3d.yaml based on template.yaml with right URLs
    cat template.yaml \
	| sed -e "s/${REGISTRY_PORT}/5000/g" \
		  -e "s/localhost/k3d-${REGISTRY_NAME}.localhost/g" \
		> template-k3d.yaml

    # Apply a marker label to be read by the lifecycle manager
    kubectl label --local=true -f ./template-k3d.yaml operator.kyma-project.io/use-local-template=true -oyaml > template-k3d-with-label.yaml

    # Deploy kyma which includes the deployment of the lifecycle-manager
    ${KYMA} alpha deploy --ci

    # Deploy the ModuleTemplate in the cluster
    kubectl apply -f template-k3d-with-label.yaml

    # Enable the module
    ${KYMA} alpha enable module ${MODULE_NAME} --channel ${MODULE_CHANNEL}

    # Wait for Telemetry CR to be in Ready state
    ./hack/verify_telemetry_status.sh

    # Wait for Kyma CR to be in Ready state
    ./hack/verify_kyma_status.sh
}

main
