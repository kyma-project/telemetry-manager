# Image URL to use all building/pushing image targets
IMG ?= europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:v20230421-c40cd7f7
# Values required for creating telemetry module
MODULE_VERSION ?= 0.0.3
MODULE_CHANNEL ?= fast
MODULE_NAME ?= telemetry
MODULE_CR_PATH ?= ./config/samples/operator_v1alpha1_telemetry.yaml
# ENVTEST_K8S_VERSION refers to the version of Kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.1
# Operating system architecture
OS_ARCH ?= $(shell uname -m)
# Operating system type
OS_TYPE ?= $(shell uname)
PROJECT_DIR ?= $(shell pwd)

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
lint-autofix: ## Autofix all possible linting errors.
	golangci-lint run -E goimports --fix

lint-manifests: controller-gen
	hack/lint-manifests.sh $(PROJECT_DIR) $(CONTROLLER_GEN)

lint: lint-manifests
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: tidy
tidy: ## Check if there any dirty change for go mod tidy.
	go mod tidy
	git diff --exit-code go.mod
	git diff --exit-code go.sum

.PHONY: test
test: manifests generate fmt vet tidy envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out
	go test -tags e2e ./test/e2e/testkit/otlp/matchers

.PHONY: e2e-test
e2e-test: ginkgo k3d ## Provision k3d cluster and run end-to-end tests.
	K8S_VERSION=$(ENVTEST_K8S_VERSION) hack/provision-test-env.sh
	$(GINKGO) run --tags e2e -v ./test/e2e
	$(K3D) cluster delete kyma
	$(K3D) registry delete k3d-kyma-registry

.PHONY: e2e-deploy-module
e2e-deploy-module: kyma kustomize ## Provision a k3d cluster and deploy module with the lifecycle manager. Manager image and module image are pushed to local k3d registry
	KYMA=${KYMA} KUSTOMIZE=${KUSTOMIZE} MODULE_NAME=${MODULE_NAME} MODULE_VERSION=${MODULE_VERSION} MODULE_CHANNEL=${MODULE_CHANNEL} MODULE_CR_PATH=${MODULE_CR_PATH} ./hack/deploy-module.sh

##@ Build

.PHONY: build
build: generate fmt vet tidy ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet tidy ## Run a controller from your host.
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
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Release Module

.PHONY: release
release: kyma kustomize ## Create module with its image pushed to prod registry and create a github release entry
	KYMA=${KYMA} KUSTOMIZE=${KUSTOMIZE} IMG=${IMG} MODULE_NAME=${MODULE_NAME} MODULE_VERSION=${MODULE_VERSION} MODULE_CHANNEL=${MODULE_CHANNEL} MODULE_CR_PATH=${MODULE_CR_PATH} ./hack/release.sh

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GINKGO ?= $(LOCALBIN)/ginkgo
K3D ?= $(LOCALBIN)/k3d
KYMA ?= $(LOCALBIN)/kyma-$(KYMA_STABILITY)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.1
CONTROLLER_TOOLS_VERSION ?= v0.11.3
K3D_VERSION ?= v5.4.7
GINKGO_VERSION ?= v2.9.4
GORELEASER_VERSION ?= v1.17.1

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

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(GINKGO) && $(GINKGO) version | grep -q $(GINKGO_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

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
