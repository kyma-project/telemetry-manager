##@ k3d
.PHONY: provision-k3d
provision-k3d: $(K3D)
	$(K3D) cluster create --image rancher/k3s:v$(strip $(K3S_K8S_VERSION))-k3s1 --config .k3d-kyma.yaml
	kubectl create ns kyma-system

.PHONY: provision-k3d-istio
provision-k3d-istio: provision-k3d
	hack/build-image.sh
	hack/deploy-istio.sh

.PHONY: cleanup-k3d
cleanup-k3d:
	$(K3D) cluster delete --config .k3d-kyma.yaml

##@ Gardener
## injected by the environment
# GARDENER_SA_PATH=
# GARDENER_PROJECT=
# GARDENER_SECRET_NAME=
GIT_COMMIT_SHA=$(shell git rev-parse --short=8 HEAD)
UNAME=$(shell uname -s)
ifeq ($(UNAME),Linux)
	export HIBERNATION_HOUR=$(shell date -d"5hours" +%-H)
endif
ifeq ($(UNAME),Darwin)
	export HIBERNATION_HOUR=$(shell date -v+5H +%-H)
endif
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
	env
	envsubst < hack/shoot_gcp.yaml > /tmp/shoot.yaml 
	cat /tmp/shoot.yaml
	cat /tmp/shoot.yaml | kubectl --kubeconfig "${GARDENER_SA_PATH}" apply -f -

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
deprovision-gardener:## Deprovision gardener cluster
	kubectl --kubeconfig=${GARDENER_SA_PATH} annotate shoot ${GARDENER_CLUSTER_NAME} confirmation.gardener.cloud/deletion=true
	kubectl --kubeconfig=${GARDENER_SA_PATH} delete shoot ${GARDENER_CLUSTER_NAME} --wait=false
