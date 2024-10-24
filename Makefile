include .env
-include .env.overrides

# Environment Variables
IMG ?= $(ENV_IMG)
K3S_K8S_VERSION ?= $(ENV_K3S_K8S_VERSION)

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
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

SRC_ROOT := $(shell git rev-parse --show-toplevel)
TOOLS_MOD_DIR    := $(SRC_ROOT)/internal/tools
TOOLS_MOD_REGEX  := "\s+_\s+\".*\""
TOOLS_PKG_NAMES  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"")
TOOLS_BIN_DIR    := $(SRC_ROOT)/bin
# Strip off versions (e.g. /v2) from pkg names
TOOLS_PKG_NAMES_CLEAN  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"" | sed "s/\/v[0-9].*$$//")
TOOLS_BIN_NAMES  := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(TOOLS_PKG_NAMES_CLEAN)))

.PHONY: install-tools
install-tools: $(TOOLS_BIN_NAMES)

$(TOOLS_BIN_DIR):
	if [ ! -d $@ ]; then mkdir -p $@; fi

$(TOOLS_BIN_NAMES): $(TOOLS_BIN_DIR) $(TOOLS_MOD_DIR)/go.mod
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES_CLEAN))%,$(TOOLS_PKG_NAMES))

CONTROLLER_GEN   := $(TOOLS_BIN_DIR)/controller-gen
GINKGO           := $(TOOLS_BIN_DIR)/ginkgo
GOLANGCI_LINT    := $(TOOLS_BIN_DIR)/golangci-lint
GO_TEST_COVERAGE := $(TOOLS_BIN_DIR)/go-test-coverage
KUSTOMIZE        := $(TOOLS_BIN_DIR)/kustomize
MOCKERY          := $(TOOLS_BIN_DIR)/mockery
TABLE_GEN        := $(TOOLS_BIN_DIR)/table-gen
YQ               := $(TOOLS_BIN_DIR)/yq
STRINGER         := $(TOOLS_BIN_DIR)/stringer
WSL				 := $(TOOLS_BIN_DIR)/wsl
K3D              := $(TOOLS_BIN_DIR)/k3d

# Sub-makefile
include hack/make/provision.mk

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
lint-fix: $(GOLANGCI_LINT) $(WSL)
	-$(WSL) --fix ./...
	$(GOLANGCI_LINT) run --fix

lint: $(GOLANGCI_LINT)
	go version
	$(GOLANGCI_LINT) version
	GO111MODULE=on $(GOLANGCI_LINT) run

.PHONY: crd-docs-gen
crd-docs-gen: $(TABLE_GEN) manifests## Generates CRD spec into docs folder
	$(TABLE_GEN) --crd-filename ./config/crd/bases/operator.kyma-project.io_telemetries.yaml --md-filename ./docs/user/resources/01-telemetry.md
	$(TABLE_GEN) --crd-filename ./config/crd/bases/telemetry.kyma-project.io_logpipelines.yaml --md-filename ./docs/user/resources/02-logpipeline.md
	$(TABLE_GEN) --crd-filename ./config/crd/bases/telemetry.kyma-project.io_logparsers.yaml --md-filename ./docs/user/resources/03-logparser.md
	$(TABLE_GEN) --crd-filename ./config/crd/bases/telemetry.kyma-project.io_tracepipelines.yaml --md-filename ./docs/user/resources/04-tracepipeline.md
	$(TABLE_GEN) --crd-filename ./config/crd/bases/telemetry.kyma-project.io_metricpipelines.yaml --md-filename ./docs/user/resources/05-metricpipeline.md

.PHONY: manifests
manifests: $(CONTROLLER_GEN) $(YQ) $(YAMLFMT) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition for v1alpha1.
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook paths="./..."
	$(CONTROLLER_GEN) crd paths="./apis/operator/v1alpha1" output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) crd paths="./apis/telemetry/v1alpha1" output:crd:artifacts:config=config/crd/bases
	$(YQ) eval 'del(.. | select(has("otlp")).otlp)' -i ./config/crd/bases/telemetry.kyma-project.io_logpipelines.yaml
	$(YQ) eval 'del(.. | select(has("x-kubernetes-validations"))."x-kubernetes-validations"[] | select(.rule|contains("otlp")) )' -i ./config/crd/bases/telemetry.kyma-project.io_logpipelines.yaml


.PHONY: manifests-dev
manifests-dev: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition for v1alpha1 and v1beta1.
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook crd paths="./..." output:crd:artifacts:config=config/development/crd/bases

.PHONY: generate
generate: $(CONTROLLER_GEN) $(MOCKERY) $(STRINGER) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(MOCKERY)
	$(STRINGER) --type Mode apis/telemetry/v1alpha1/logpipeline_types.go
	$(STRINGER) --type Mode apis/telemetry/v1beta1/logpipeline_types.go
	$(STRINGER) --type FeatureFlag internal/featureflags/featureflags.go
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


##@ Testing
.PHONY: test
test: $(GINKGO) manifests generate fmt vet tidy ## Run tests.
	$(GINKGO) run test/testkit/matchers/...
	go test ./... -coverprofile cover.out

.PHONY: check-coverage
check-coverage: $(GO_TEST_COVERAGE) ## Check tests coverage.
	go test ./... -short -coverprofile=cover.out -covermode=atomic -coverpkg=./...
	$(GO_TEST_COVERAGE) --config=./.testcoverage.yml


##@ Build
.PHONY: build
build: generate fmt vet tidy ## Build manager binary.
	go build -o bin/manager main.go

check-clean: ## Check if repo is clean up-to-date. Used after code generation
	@echo "Checking if all generated files are up-to-date"
	@git diff --name-only --exit-code || (echo "Generated files are not up-to-date. Please run 'make generate manifests manifests-dev' to update them." && exit 1)


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
install: manifests $(KUSTOMIZE) ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: install-with-telemetry
install-with-telemetry: install
	kubectl get ns kyma-system || kubectl create ns kyma-system
	kubectl apply -f config/samples/operator_v1alpha1_telemetry.yaml -n kyma-system

.PHONY: uninstall
uninstall: manifests $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests $(KUSTOMIZE) ## Deploy resources based on the release (default) variant to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: $(KUSTOMIZE) ## Undeploy resources based on the release (default) variant from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-dev
deploy-dev: manifests-dev $(KUSTOMIZE) ## Deploy resources based on the development variant to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/development | kubectl apply -f -

.PHONY: undeploy-dev
undeploy-dev: $(KUSTOMIZE) ## Undeploy resources based on the development variant from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/development | kubectl delete --ignore-not-found=$(ignore-not-found) -f -
