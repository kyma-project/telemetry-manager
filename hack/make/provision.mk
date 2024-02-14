##@ Gardener
## injected by the environment
# GARDENER_SA_PATH=
# GARDENER_PROJECT=
# GARDENER_SECRET_NAME=
GIT_COMMIT_SHA=$(shell git rev-parse --short=8 HEAD)
HIBERNATION_HOUR=$(shell echo $$(( ( $(shell date +%H | sed s/^0//g) + 5 ) % 24 )))
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