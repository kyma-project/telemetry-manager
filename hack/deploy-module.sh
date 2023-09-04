#!/usr/bin/env bash

readonly CLUSTER_NAME="${CLUSTER_NAME:-kyma}" 
readonly REGISTRY_NAME="${REGISTRY_NAME:-${CLUSTER_NAME}-registry}"
readonly REGISTRY_PORT="${REGISTRY_PORT:-5001}" 
readonly MODULE_REGISTRY="${MODULE_REGISTRY:-localhost:${REGISTRY_PORT}}"

function build_and_push_manager_image() {
    export IMG=localhost:${REGISTRY_PORT}/${MODULE_NAME}-manager
    make docker-build
    make docker-push
}

function create_module() {
    export IMG=k3d-${REGISTRY_NAME}:${REGISTRY_PORT}/${MODULE_NAME}-manager
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    # create module command uses Kustomization files defined in "config/default"
    ${KYMA} alpha create module --name kyma-project.io/module/${MODULE_NAME} --version ${MODULE_VERSION} --channel ${MODULE_CHANNEL} --default-cr ${MODULE_CR_PATH} --registry ${MODULE_REGISTRY} --insecure --ci --kubebuilder-project
}

function apply_local_template_label() {
    kubectl label --local=true -f template.yaml operator.kyma-project.io/use-local-template=true -o yaml > temporary-template.yaml
    mv temporary-template.yaml template.yaml
}

function verify_telemetry_status() {
	local number=1
	while [[ $number -le 100 ]] ; do
		echo ">--> checking telemetry status #$number"
		local STATUS=$(kubectl get telemetry -n kyma-system default -o jsonpath='{.status.state}')
		echo "telemetry status: ${STATUS:='UNKNOWN'}"
		[[ "$STATUS" == "Ready" ]] && return 0
		sleep 15
        	((number = number + 1))
	done

	kubectl get all --all-namespaces
	exit 1
}

function verify_kyma_status() {
	local number=1
	while [[ $number -le 100 ]] ; do
		echo ">--> checking kyma status #$number"
		local STATUS=$(kubectl get kyma -n kyma-system default-kyma -o jsonpath='{.status.state}')
		echo "kyma status: ${STATUS:='UNKNOWN'}"
		[[ "$STATUS" == "Ready" ]] && return 0
		sleep 15
        	((number = number + 1))
	done

	kubectl get all --all-namespaces
	exit 1
}

function main() {
    # Provision a k3d cluster using Kyma cli
    ${KYMA} provision k3d --registry-port ${REGISTRY_PORT} --name ${CLUSTER_NAME} --ci
    
    # Build and push manager image to a local k3d registry
    build_and_push_manager_image
    
    # Create the module and push its image to a local k3d registry
    create_module

    # Modify template.yaml with the URL needed for lifecycle manager to access the module image from inside the k3d cluster
    sed -e "s/${REGISTRY_PORT}/5000/" \
		-e "s/localhost/k3d-${REGISTRY_NAME}.localhost/" \
        -i "" template.yaml
	
    # Apply label needed by the lifecycle manager for local module deployment
    apply_local_template_label

    # Deploy kyma which includes the deployment of the lifecycle-manager
    ${KYMA} alpha deploy --ci

    # Deploy the ModuleTemplate in the cluster
    kubectl apply -f template.yaml

    # Enable the module
    ${KYMA} alpha enable module ${MODULE_NAME} --channel ${MODULE_CHANNEL}

    # Wait for Telemetry CR to be in Ready state
    verify_telemetry_status

    # Wait for Kyma CR to be in Ready state
    verify_kyma_status
}

main
