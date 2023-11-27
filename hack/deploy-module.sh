#!/usr/bin/env bash

readonly REGISTRY_NAME="${REGISTRY_NAME:-kyma-registry}"
readonly REGISTRY_PORT="${REGISTRY_PORT:-5001}"
readonly MODULE_REGISTRY="${MODULE_REGISTRY:-localhost:${REGISTRY_PORT}}"
readonly IMG="${IMG:-k3d-${REGISTRY_NAME}:${REGISTRY_PORT}/telemetry-manager}"

function create_module() {
    cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../..
    ${KUSTOMIZE} build config/default > telemetry-manager.yaml
    git remote add origin https://github.com/kyma-project/telemetry-manager
    ${KYMA} alpha create module \
    --module-config-file=module-config.yaml \
    --registry ${MODULE_REGISTRY} \
    --insecure \
    --output moduletemplate.yaml \
    --module-archive-version-overwrite \
    --ci
}

function apply_local_template_label() {
    kubectl label --local=true -f moduletemplate.yaml operator.kyma-project.io/use-local-template=true -o yaml > temporary-template.yaml
    mv temporary-template.yaml moduletemplate.yaml
}

function verify_telemetry_status() {
	local number=1
	while [[ $number -le 20 ]] ; do
		echo ">--> checking telemetry status #$number"
		local STATUS=$(kubectl get telemetry -n kyma-system default -o jsonpath='{.status.state}')
		echo "telemetry status: ${STATUS:='UNKNOWN'}"
		[[ "$STATUS" == "Ready" ]] && return 0
		sleep 15
        	((number = number + 1))
	done

	exit 1
}

function verify_kyma_status() {
	local number=1
	while [[ $number -le 20 ]] ; do
		echo ">--> checking kyma status #$number"
		local STATUS=$(kubectl get kyma -n kyma-system default-kyma -o jsonpath='{.status.state}')
		echo "kyma status: ${STATUS:='UNKNOWN'}"
		[[ "$STATUS" == "Ready" ]] && return 0
		sleep 15
        	((number = number + 1))
	done

	exit 1
}

function main() {
    ${KYMA} version

    # Create the module and push its image to a local k3d registry
    create_module

    # Modify moduletemplate.yaml with the URL needed for lifecycle manager to access the module image from inside the k3d cluster
    sed -e "s/${REGISTRY_PORT}/5000/" \
		-e "s/localhost/k3d-${REGISTRY_NAME}.localhost/" \
        -i "" moduletemplate.yaml

    # Apply label needed by the lifecycle manager for local module deployment
    apply_local_template_label

    # Deploy kyma which includes the deployment of the lifecycle-manager
    ${KYMA} alpha deploy --ci

    # Deploy the ModuleTemplate in the cluster
    kubectl apply -f moduletemplate.yaml

    # Enable the module
    ${KYMA} alpha enable module telemetry --channel fast

    # Wait for Telemetry CR to be in Ready state
    verify_telemetry_status

    # Wait for Kyma CR to be in Ready state
    verify_kyma_status
}

main
