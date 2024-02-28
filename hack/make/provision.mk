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
HIBERNATION_HOUR=$(shell echo $$(( ( $(shell date +%H | sed s/^0//g) + 5 ) % 24 )))
GARDENER_K8S_VERSION ?= $(ENV_GARDENER_K8S_VERSION)
GARDENER_CLUSTER_NAME=$(shell echo "ci-${GIT_COMMIT_SHA}-${GARDENER_K8S_VERSION}" | sed 's/\.//g')
ifneq (,$(GARDENER_SA_PATH))
GARDENER_K8S_VERSION_FULL=$(shell kubectl --kubeconfig=${GARDENER_SA_PATH} get cloudprofiles.core.gardener.cloud gcp -o go-template='{{range .spec.kubernetes.versions}}{{if and (eq .classification "supported") (lt .version "${GARDENER_K8S_VERSION}.a") (gt .version "${GARDENER_K8S_VERSION}")}}{{.version}}{{end}}{{end}}')
endif

.PHONY: provision-gardener
provision-gardener: kyma ## Provision gardener cluster with latest k8s version
	${KYMA} provision gardener gcp -c ${GARDENER_SA_PATH} -n ${GARDENER_CLUSTER_NAME} -p ${GARDENER_PROJECT} -s ${GARDENER_SECRET_NAME} -k ${GARDENER_K8S_VERSION_FULL} --hibernation-start="00 ${HIBERNATION_HOUR} * * ?"

.PHONY: deprovision-gardener
deprovision-gardener: kyma ## Deprovision gardener cluster
	kubectl --kubeconfig=${GARDENER_SA_PATH} annotate shoot ${GARDENER_CLUSTER_NAME} confirmation.gardener.cloud/deletion=true
	kubectl --kubeconfig=${GARDENER_SA_PATH} delete shoot ${GARDENER_CLUSTER_NAME} --wait=false