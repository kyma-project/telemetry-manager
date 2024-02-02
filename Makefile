# Image URL to use all building/pushing image targets
IMG ?= europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:main
# ENVTEST_K8S_VERSION refers to the version of Kubebuilder assets to be downloaded by envtest binary.

ENVTEST_K8S_VERSION = 1.27.1
GARDENER_K8S_VERSION ?= 1.27
ISTIO_VERSION ?= 1.3.0

# Operating system architecture
OS_ARCH ?= $(shell uname -m)
# Operating system type
OS_TYPE ?= $(shell uname)
PROJECT_DIR ?= $(shell pwd)
ARTIFACTS ?= $(shell pwd)/artifacts

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
lint-autofix: golangci-lint ## Autofix all possible linting errors.
	${GOLANGCI-LINT} run --fix

lint-manifests:
	hack/lint-manifests.sh

lint: golangci-lint lint-manifests
	go version
	${GOLANGCI-LINT} version
	GO111MODULE=on ${GOLANGCI-LINT} run

.PHONY: crd-docs-gen
crd-docs-gen: tablegen ## Generates CRD spec into docs folder
	${TABLE_GEN} --crd-filename ./config/crd/bases/operator.kyma-project.io_telemetries.yaml --md-filename ./docs/user/resources/01-telemetry.md
	${TABLE_GEN} --crd-filename ./config/crd/bases/telemetry.kyma-project.io_logpipelines.yaml --md-filename ./docs/user/resources/02-logpipeline.md
	${TABLE_GEN} --crd-filename ./config/crd/bases/telemetry.kyma-project.io_logparsers.yaml --md-filename ./docs/user/resources/03-logparser.md
	${TABLE_GEN} --crd-filename ./config/crd/bases/telemetry.kyma-project.io_tracepipelines.yaml --md-filename ./docs/user/resources/04-tracepipeline.md
	${TABLE_GEN} --crd-filename ./config/crd/bases/telemetry.kyma-project.io_metricpipelines.yaml --md-filename ./docs/user/resources/05-metricpipeline.md

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=operator-manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(MAKE) crd-docs-gen

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet --tags e2e,istio ./...

.PHONY: tidy
tidy: ## Check if there any dirty change for go mod tidy.
	go mod tidy
	git diff --exit-code go.mod
	git diff --exit-code go.sum

.PHONY: test
test: manifests generate fmt vet tidy envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

test-matchers: ginkgo
	$(GINKGO) run ./test/testkit/matchers/...

.PHONY: provision-test-env
provision-test-env: provision-k3d
	K8S_VERSION=$(ENVTEST_K8S_VERSION) hack/build-image.sh

.PHONY: provision-k3d
provision-k3d: k3d
	K8S_VERSION=$(ENVTEST_K8S_VERSION) hack/provision-k3d.sh

.PHONY: e2e-test-logs
e2e-test-logs: provision-test-env ## Provision k3d cluster, deploy development variant and run end-to-end logs tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev
	make run-e2e-test-logs

.PHONY: e2e-test-traces
e2e-test-traces: provision-test-env ## Provision k3d cluster, deploy development variant and run end-to-end traces tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev
	make run-e2e-test-traces

.PHONY: e2e-test-metrics
e2e-test-metrics: provision-test-env ## Provision k3d cluster, deploy development variant and run end-to-end metrics tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev
	make run-e2e-test-metrics

.PHONY: e2e-test-telemetry
e2e-test-telemetry: provision-test-env ## Provision k3d cluster, deploy development variant and run end-to-end telemetry tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev
	make run-e2e-test-telemetry

.PHONY: e2e-test-logs-release
e2e-test-logs-release: provision-test-env ## Provision k3d cluster, deploy release (default) variant and run end-to-end logs tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
	make run-e2e-test-logs

.PHONY: e2e-test-traces-release
e2e-test-traces-release: provision-test-env ## Provision k3d cluster, deploy release (default) variant and run end-to-end traces tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
	make run-e2e-test-traces

.PHONY: e2e-test-metrics-release
e2e-test-metrics-release: provision-test-env ## Provision k3d cluster, deploy release (default) variant and run end-to-end metrics tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
	make run-e2e-test-metrics

.PHONY: e2e-test-telemetry-release
e2e-test-telemetry-release: provision-test-env ## Provision k3d cluster, deploy release (default) variant and run end-to-end telemetry tests.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
	make run-e2e-test-telemetry

.PHONY: run-e2e-test-logs
run-e2e-test-logs: ginkgo test-matchers ## run end-to-end logs tests using an existing cluster
	$(GINKGO) run --tags e2e --junit-report=junit.xml --label-filter="logs" ./test/e2e
	mkdir -p ${ARTIFACTS}
	mv junit.xml ${ARTIFACTS}

.PHONY: run-e2e-test-traces
run-e2e-test-traces: ginkgo test-matchers ## run end-to-end traces tests using an existing cluster
	$(GINKGO) run --tags e2e --junit-report=junit.xml --label-filter="traces" ./test/e2e
	mkdir -p ${ARTIFACTS}
	mv junit.xml ${ARTIFACTS}

.PHONY: run-e2e-test-metrics
run-e2e-test-metrics: ginkgo test-matchers ## run end-to-end metrics tests using an existing cluster
	$(GINKGO) run --tags e2e --junit-report=junit.xml --label-filter="metrics" ./test/e2e
	mkdir -p ${ARTIFACTS}
	mv junit.xml ${ARTIFACTS}

.PHONY: run-e2e-test-telemetry
run-e2e-test-telemetry: ginkgo test-matchers ## run end-to-end telemetry tests using an existing cluster
	$(GINKGO) run --tags e2e --junit-report=junit.xml --label-filter="telemetry" ./test/e2e
	mkdir -p ${ARTIFACTS}
	mv junit.xml ${ARTIFACTS}

.PHONY: upgrade-test
upgrade-test: provision-k3d ## Provision k3d cluster and run upgrade tests.
	hack/upgrade-test.sh

.PHONY: run-upgrade-test
run-upgrade-test: ginkgo
	$(GINKGO) run --tags e2e --junit-report=junit.xml --flake-attempts=5 --label-filter="operational" -v ./test/e2e
	mkdir -p ${ARTIFACTS}
	mv junit.xml ${ARTIFACTS}

.PHONY: e2e-deploy-module
e2e-deploy-module: kyma kustomize provision-k3d provision-test-env ## Provision a k3d cluster and deploy module with the lifecycle manager. Manager image and module image are pushed to local k3d registry
	make run-e2e-deploy-module

.PHONY: run-e2e-deploy-module
run-e2e-deploy-module: kyma kustomize ## Deploy module with the lifecycle manager.
	KYMA=${KYMA} KUSTOMIZE=${KUSTOMIZE} ./hack/deploy-module.sh

.PHONY: integration-test-istio
integration-test-istio: ginkgo k3d | test-matchers provision-test-env ## Provision k3d cluster, deploy development variant and run integration tests with istio.
	IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev
	make run-integration-test-istio

.PHONY: run-integration-test-istio
run-integration-test-istio: ginkgo test-matchers ## run integration tests with istio on an existing cluster
	ISTIO_VERSION=$(ISTIO_VERSION) hack/deploy-istio.sh
	$(GINKGO) run --tags istio --junit-report=junit.xml ./test/integration/istio
	mkdir -p ${ARTIFACTS}
	mv junit.xml ${ARTIFACTS}

.PHONY: check-coverage
check-coverage: go-test-coverage
	go test ./... -short -coverprofile=cover.out -covermode=atomic -coverpkg=./...
	$(GO_TEST_COVERAGE) --config=./.testcoverage.yml

##@ Build

.PHONY: build
build: generate fmt vet tidy ## Build manager binary.
	go build -o bin/manager main.go

tls.key:
	@openssl genrsa -out tls.key 4096

tls.crt: tls.key
	@openssl req -sha256 -new -key tls.key -out tls.csr -subj '/CN=localhost'
	@openssl x509 -req -sha256 -days 3650 -in tls.csr -signkey tls.key -out tls.crt
	@rm tls.csr

gen-webhook-cert: tls.key tls.crt

.PHONY: run
run: gen-webhook-cert manifests generate fmt vet tidy ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy resources based on the release (default) variant to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy resources based on the release (default) variant from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-dev
deploy-dev: manifests kustomize ## Deploy resources based on the development variant to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/development | kubectl apply -f -

.PHONY: undeploy-dev
undeploy-dev: ## Undeploy resources based on the development variant from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/development | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Release Module

.PHONY: release
release: kustomize ## Prepare release artefacts and create a GitHub release
	KUSTOMIZE=${KUSTOMIZE} IMG=${IMG} GORELEASER_VERSION=${GORELEASER_VERSION} ./hack/release.sh

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
TABLE_GEN ?= $(LOCALBIN)/table-gen
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GINKGO ?= $(LOCALBIN)/ginkgo
GOLANGCI-LINT ?= $(LOCALBIN)/golangci-lint
GO_TEST_COVERAGE ?= $(LOCALBIN)/go-test-coverage
K3D ?= $(LOCALBIN)/k3d
KYMA ?= $(LOCALBIN)/kyma-$(KYMA_STABILITY)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.1
TABLE_GEN_VERSION ?= v0.0.0-20230523174756-3dae9f177ffd
CONTROLLER_TOOLS_VERSION ?= v0.11.3
K3D_VERSION ?= v5.4.7
GINKGO_VERSION ?= v2.15.0
GORELEASER_VERSION ?= v1.23.0
GOLANGCI-LINT_VERSION ?= latest
GO_TEST_COVERAGE_VERSION ?= v2.8.2

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(KUSTOMIZE) && ! $(KUSTOMIZE) version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(KUSTOMIZE) version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(KUSTOMIZE); \
	fi
	test -s $(KUSTOMIZE) || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) --output install_kustomize.sh && bash install_kustomize.sh $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); rm install_kustomize.sh; }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(ENVTEST) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: tablegen
tablegen: $(TABLE_GEN) ## Download table-gen locally if necessary.
$(TABLE_GEN): $(LOCALBIN)
	test -s $(TABLE_GEN) || GOBIN=$(LOCALBIN) go install github.com/kyma-project/kyma/hack/table-gen@$(TABLE_GEN_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI-LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI-LINT): $(LOCALBIN)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI-LINT_VERSION)

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(GINKGO) && $(GINKGO) version | grep -q $(GINKGO_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: go-test-coverage
go-test-coverage: $(GO_TEST_COVERAGE) ## Download go-test-coverage locally if necessary.
$(GO_TEST_COVERAGE): $(LOCALBIN)
	test -s $(GO_TEST_COVERAGE) && $(GO_TEST_COVERAGE) --version | grep -q $(GO_TEST_COVERAGE_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/vladopajic/go-test-coverage/v2@$(GO_TEST_COVERAGE_VERSION)

K3D_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh"
.PHONY: k3d
k3d: $(K3D) ## Download k3d locally if necessary. If wrong version is installed, it will be removed before downloading.
$(K3D): $(LOCALBIN)
	@if test -x $(K3D) && ! $(K3D) version | grep -q $(K3D_VERSION); then \
		echo "$(K3D) version is not as expected '$(K3D_VERSION)'. Removing it before installing."; \
		rm -rf $(K3D); \
	fi
	test -s $(K3D) || curl -s $(K3D_INSTALL_SCRIPT) | PATH="$(PATH):$(LOCALBIN)" USE_SUDO=false K3D_INSTALL_DIR=$(LOCALBIN) TAG=$(K3D_VERSION) bash

define os_error
$(error Error: unsupported platform OS_TYPE:$1, OS_ARCH:$2; to mitigate this problem set variable KYMA with absolute path to kyma-cli binary compatible with your operating system and architecture)
endef

KYMA_FILENAME ?=  $(shell ./hack/get-kyma-filename.sh ${OS_TYPE} ${OS_ARCH})
KYMA_STABILITY ?= unstable

.PHONY: kyma
kyma: $(LOCALBIN) $(KYMA) ## Download Kyma cli locally if necessary.
$(KYMA):
	$(if $(KYMA_FILENAME),,$(call os_error, ${OS_TYPE}, ${OS_ARCH}))
	test -f $@ || curl -s -Lo $(KYMA) https://storage.googleapis.com/kyma-cli-$(KYMA_STABILITY)/$(KYMA_FILENAME)
	chmod 0100 $(KYMA)

##@ Gardener
## injected by the environment
# GARDENER_SA_PATH=
# GARDENER_PROJECT=
# GARDENER_SECRET_NAME=
GIT_COMMIT_SHA=$(shell git rev-parse --short=8 HEAD)
GIT_COMMIT_DATE=$(shell git show -s --format=%cd --date=format:'v%Y%m%d' ${GIT_COMMIT_SHA})
HIBERNATION_HOUR=$(shell echo $$(( ( $(shell date +%H | sed s/^0//g) + 5 ) % 24 )))

ifneq (,$(GARDENER_SA_PATH))
GARDENER_K8S_VERSION_FULL?=$(shell kubectl --kubeconfig=${GARDENER_SA_PATH} get cloudprofiles.core.gardener.cloud gcp -o go-template='{{range .spec.kubernetes.versions}}{{if and (eq .classification "supported") (lt .version "${GARDENER_K8S_VERSION}.a") (gt .version "${GARDENER_K8S_VERSION}")}}{{.version}}{{end}}{{end}}')
endif

# log tests are excluded for now as they are too flaky
.PHONY: run-tests-with-git-image
run-tests-with-git-image: ## Run e2e tests on existing cluster using image related to git commit sha
	kubectl create namespace kyma-system
	IMG=europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:${GIT_COMMIT_DATE}-${GIT_COMMIT_SHA} make deploy-dev
	make run-integration-test-istio

.PHONY: gardener-integration-test
gardener-integration-test: ## Provision gardener cluster and run integration test on it.
	make provision-gardener \
		run-tests-with-git-image \
		deprovision-gardener || \
		(make deprovision-gardener && false)

.PHONY: provision-gardener
provision-gardener: kyma ## Provision gardener cluster with latest k8s version
	${KYMA} provision gardener gcp -c ${GARDENER_SA_PATH} -n test-${GIT_COMMIT_SHA} -p ${GARDENER_PROJECT} -s ${GARDENER_SECRET_NAME} -k ${GARDENER_K8S_VERSION_FULL}\
		--hibernation-start="00 ${HIBERNATION_HOUR} * * ?"

.PHONY: deprovision-gardener
deprovision-gardener: kyma ## Deprovision gardener cluster
	kubectl --kubeconfig=${GARDENER_SA_PATH} annotate shoot test-${GIT_COMMIT_SHA} confirmation.gardener.cloud/deletion=true
	kubectl --kubeconfig=${GARDENER_SA_PATH} delete shoot test-${GIT_COMMIT_SHA} --wait=false
