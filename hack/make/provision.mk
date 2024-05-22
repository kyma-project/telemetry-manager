##@ k3d
.PHONY: provision-k3d
provision-k3d: k3d
	K8S_VERSION=$(ENVTEST_K8S_VERSION) hack/provision-k3d.sh

.PHONY: provision-k3d-e2e
provision-k3d-e2e: kyma kustomize provision-k3d ## Provision a k3d cluster and deploy module with the lifecycle manager. Manager image and module image are pushed to local k3d registry
	K8S_VERSION=$(ENVTEST_K8S_VERSION) hack/build-image.sh
	KYMA=${KYMA} KUSTOMIZE=${KUSTOMIZE} hack/deploy-module.sh

.PHONY: provision-k3d-istio
provision-k3d-istio: provision-k3d
	K8S_VERSION=$(ENVTEST_K8S_VERSION) hack/build-image.sh
	hack/deploy-istio.sh


##@ Gardener
## injected by the environment
# GARDENER_SA_PATH=
# GARDENER_PROJECT=
# GARDENER_SECRET_NAME=
GIT_COMMIT_SHA=$(shell git rev-parse --short=8 HEAD)
export HIBERNATION_HOUR=$(shell echo $$(( ( $(shell date +%H | sed s/^0//g) + 5 ) % 24 )))
GARDENER_K8S_VERSION ?= $(ENV_GARDENER_K8S_VERSION)
# Cluster name is also set via load test. If its set then use that else use ci-XX
export GARDENER_CLUSTER_NAME ?= $(shell echo "ci-${GIT_COMMIT_SHA}-${GARDENER_K8S_VERSION}" | sed 's/\.//g')
export GARDENER_MACHINE_TYPE ?= $(ENV_GARDENER_MACHINE_TYPE)
export GARDENER_MIN_NODES ?= $(ENV_GARDENER_MIN_NODES)
export GARDENER_MAX_NODES ?= $(ENV_GARDENER_MAX_NODES)

ifneq (,$(GARDENER_SA_PATH))
export GARDENER_K8S_VERSION_FULL=$(shell kubectl --kubeconfig=${GARDENER_SA_PATH} get cloudprofiles.core.gardener.cloud gcp -o go-template='{{range .spec.kubernetes.versions}}{{if and (eq .classification "supported") (lt .version "${GARDENER_K8S_VERSION}.a") (gt .version "${GARDENER_K8S_VERSION}")}}{{.version}}{{break}}{{end}}{{end}}')
endif

.PHONY: provision-gardener
provision-gardener: ## Provision gardener cluster with latest k8s version
	envsubst < hack/shoot_gcp.yaml | kubectl --kubeconfig "${GARDENER_SA_PATH}" apply -f -

	echo "waiting fo cluster to be ready..."
	kubectl wait --kubeconfig "${GARDENER_SA_PATH}" --for=condition=EveryNodeReady shoot/${GARDENER_CLUSTER_NAME} --timeout=17m

	# create kubeconfig request, that creates a kubeconfig which is valid for one day
	kubectl --kubeconfig "${GARDENER_SA_PATH}" create \
		-f <(printf '{"spec":{"expirationSeconds":86400}}') \
		--raw /apis/core.gardener.cloud/v1beta1/namespaces/garden-${GARDENER_PROJECT}/shoots/${GARDENER_CLUSTER_NAME}/adminkubeconfig | \
		jq -r ".status.kubeconfig" | \
		base64 -d > ${GARDENER_CLUSTER_NAME}_kubeconfig.yaml

	# replace the default kubeconfig
	mkdir -p ~/.kube
	mv ${GARDENER_CLUSTER_NAME}_kubeconfig.yaml ~/.kube/config

.PHONY: deprovision-gardener
deprovision-gardener: kyma ## Deprovision gardener cluster
	kubectl --kubeconfig=${GARDENER_SA_PATH} annotate shoot ${GARDENER_CLUSTER_NAME} confirmation.gardener.cloud/deletion=true
	kubectl --kubeconfig=${GARDENER_SA_PATH} delete shoot ${GARDENER_CLUSTER_NAME} --wait=false
