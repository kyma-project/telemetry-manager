include .env
-include .env.overrides

# Environment Variables
MANAGER_IMAGE ?= $(ENV_MANAGER_IMAGE)
FLUENT_BIT_EXPORTER_IMAGE?= $(ENV_FLUENTBIT_EXPORTER_IMAGE)
FLUENT_BIT_IMAGE ?= $(ENV_FLUENTBIT_IMAGE)
OTEL_COLLECTOR_IMAGE ?= $(ENV_OTEL_COLLECTOR_IMAGE)
SELF_MONITOR_IMAGE?= $(ENV_SELFMONITOR_IMAGE)
K3S_IMAGE ?= $(ENV_K3S_IMAGE)

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
DEPENDENCIES_DIR := $(SRC_ROOT)/dependencies
TOOLS_MOD_DIR    := $(SRC_ROOT)/internal/tools
TOOLS_MOD_REGEX  := "\s+_\s+\".*\""
TOOLS_PKG_NAMES  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"")
TOOLS_BIN_DIR    := $(SRC_ROOT)/bin
# Strip off versions (e.g. /v2) from pkg names
TOOLS_PKG_NAMES_CLEAN  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"" | sed "s/\/v[0-9].*$$//")
TOOLS_BIN_NAMES  := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(TOOLS_PKG_NAMES_CLEAN)))

.PHONY: install-tools
install-tools: $(TOOLS_BIN_NAMES) $(POPULATE_IMAGES) $(PROMLINTER)

$(TOOLS_BIN_DIR):
	if [ ! -d $@ ]; then mkdir -p $@; fi

$(TOOLS_BIN_NAMES): $(TOOLS_BIN_DIR) $(TOOLS_MOD_DIR)/go.mod
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES_CLEAN))%,$(TOOLS_PKG_NAMES))

CONTROLLER_GEN   := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCI_LINT    := $(TOOLS_BIN_DIR)/golangci-lint
GO_TEST_COVERAGE := $(TOOLS_BIN_DIR)/go-test-coverage
KUSTOMIZE        := $(TOOLS_BIN_DIR)/kustomize
MOCKERY          := $(TOOLS_BIN_DIR)/mockery
TABLE_GEN        := $(TOOLS_BIN_DIR)/table-gen
YQ               := $(TOOLS_BIN_DIR)/yq
JQ               := $(TOOLS_BIN_DIR)/gojq
YAMLFMT          := $(TOOLS_BIN_DIR)/yamlfmt
STRINGER         := $(TOOLS_BIN_DIR)/stringer
WSL              := $(TOOLS_BIN_DIR)/wsl
K3D              := $(TOOLS_BIN_DIR)/k3d
PROMLINTER       := $(TOOLS_BIN_DIR)/promlinter
GOMPLATE         := $(TOOLS_BIN_DIR)/gomplate

POPULATE_IMAGES  := $(TOOLS_BIN_DIR)/populate-images

.PHONY: $(POPULATE_IMAGES)
$(POPULATE_IMAGES):
	cd $(DEPENDENCIES_DIR)/populateimages && go build -o $(POPULATE_IMAGES) main.go

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



# Find dependency folders that contain go.mod
GO_MODULE_DIRS := $(shell find $(DEPENDENCIES_DIR) -mindepth 1 -maxdepth 1 -type d -exec test -f "{}/go.mod" \; -print)
MODULE_NAMES := $(notdir $(GO_MODULE_DIRS))

# All standard and fix lint targets
LINT_TARGETS := $(addprefix lint-,$(MODULE_NAMES))
LINT_FIX_TARGETS := $(addprefix lint-fix-,$(MODULE_NAMES))

# Declare phony targets for shell completion

# Lint the root module
lint-manager: $(GOLANGCI_LINT)
	@echo "Linting root module..."
	@$(GOLANGCI_LINT) run --config $(SRC_ROOT)/.golangci.yaml

# Lint the root module with --fix
lint-fix-manager: $(GOLANGCI_LINT)
	@echo "Linting root module (with fix)..."
	@$(GOLANGCI_LINT) run --config $(SRC_ROOT)/.golangci.yaml --fix

# Pattern rule for standard lint targets
$(LINT_TARGETS): $(GOLANGCI_LINT)
	@modname=$(@:lint-%=%); \
	echo "Linting $$modname..."; \
	cd $(DEPENDENCIES_DIR)/$$modname && $(GOLANGCI_LINT) run --config $(SRC_ROOT)/.golangci.yaml

# Pattern rule for fix lint targets
$(LINT_FIX_TARGETS): $(GOLANGCI_LINT)
	@modname=$(@:lint-fix-%=%); \
	echo "Linting $$modname (with fix)..."; \
	cd $(DEPENDENCIES_DIR)/$$modname && $(GOLANGCI_LINT) run --config $(SRC_ROOT)/.golangci.yaml --fix

# Lint everything
lint: lint-manager $(LINT_TARGETS)
	@echo "All lint checks completed."

# Lint everything with fix
lint-fix: lint-fix-manager $(LINT_FIX_TARGETS)
	@echo "All lint fix checks completed."

.PHONY: lint-manager lint-fix-manager lint lint-fix $(LINT_TARGETS) $(LINT_FIX_TARGETS)

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
# Strip off transform field from the CRDs until the feature is fully implemented
	$(YQ) eval 'del(.. | select(has("transform")).transform)' -i ./config/crd/bases/telemetry.kyma-project.io_logpipelines.yaml
	$(YQ) eval 'del(.. | select(has("transform")).transform)' -i ./config/crd/bases/telemetry.kyma-project.io_tracepipelines.yaml
	$(YQ) eval 'del(.. | select(has("transform")).transform)' -i ./config/crd/bases/telemetry.kyma-project.io_metricpipelines.yaml
	$(YQ) eval 'del(.. | select(has("x-kubernetes-validations"))."x-kubernetes-validations"[] | select(.rule|contains("transform")) )' -i ./config/crd/bases/telemetry.kyma-project.io_logpipelines.yaml
	$(YQ) eval 'del(.. | select(has("x-kubernetes-validations"))."x-kubernetes-validations"[] | select(.rule|contains("transform")) )' -i ./config/crd/bases/telemetry.kyma-project.io_metricpipelines.yaml
	$(YQ) eval 'del(.. | select(has("x-kubernetes-validations"))."x-kubernetes-validations"[] | select(.rule|contains("transform")) )' -i ./config/crd/bases/telemetry.kyma-project.io_tracepipelines.yaml
	$(YAMLFMT)

.PHONY: manifests-experimental
manifests-experimental: $(CONTROLLER_GEN) $(YAMLFMT) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition for v1alpha1 and v1beta1.
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook crd paths="./..." output:crd:artifacts:config=config/development/crd/bases
	$(YAMLFMT)


.PHONY: generate
generate: $(CONTROLLER_GEN) $(MOCKERY) $(STRINGER) $(YQ) $(YAMLFMT) $(POPULATE_IMAGES) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(MOCKERY)
	$(STRINGER) --type Mode internal/utils/logpipeline/logpipeline.go
	$(STRINGER) --type FeatureFlag internal/featureflags/featureflags.go
	$(YQ) eval '.spec.template.spec.containers[] |= (select(.name == "manager") | .env[] |= (select(.name == "FLUENT_BIT_IMAGE") | .value = ${FLUENT_BIT_IMAGE}))' -i config/manager/manager.yaml
	$(YQ) eval '.spec.template.spec.containers[] |= (select(.name == "manager") | .env[] |= (select(.name == "FLUENT_BIT_EXPORTER_IMAGE") | .value = ${FLUENT_BIT_EXPORTER_IMAGE}))' -i config/manager/manager.yaml
	$(YQ) eval '.spec.template.spec.containers[] |= (select(.name == "manager") | .env[] |= (select(.name == "OTEL_COLLECTOR_IMAGE") | .value = ${OTEL_COLLECTOR_IMAGE}))' -i config/manager/manager.yaml
	$(YQ) eval '.spec.template.spec.containers[] |= (select(.name == "manager") | .env[] |= (select(.name == "SELF_MONITOR_IMAGE") | .value = ${SELF_MONITOR_IMAGE}))' -i config/manager/manager.yaml
	$(YAMLFMT)
	$(POPULATE_IMAGES)
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
test: manifests generate fmt vet tidy ## Run tests.
	go test ./test/testkit/matchers/...
	go test $$(go list ./... | grep -v /test/) -coverprofile cover.out

.PHONY: check-coverage
check-coverage: $(GO_TEST_COVERAGE) ## Check tests coverage.
	go test $$(go list ./... | grep -v /test/) -short -coverprofile=cover.out -covermode=atomic -coverpkg=./...
	$(GO_TEST_COVERAGE) --config=./.testcoverage.yml


##@ Build
.PHONY: build
build: generate fmt vet tidy ## Build manager binary.
	go build -o bin/manager main.go

check-clean: generate manifests manifests-experimental crd-docs-gen ## Check if repo is clean up-to-date. Used after code generation
	@echo "Checking if all generated files are up-to-date"
	@git diff --name-only --exit-code || (echo "Generated files are not up-to-date. Please run 'make generate manifests manifests-experimental crd-docs-gen' to update them." && exit 1)


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
	docker build -t ${MANAGER_IMAGE} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${MANAGER_IMAGE}


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
	cd config/manager && $(KUSTOMIZE) edit set image controller=${MANAGER_IMAGE}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: $(KUSTOMIZE) ## Undeploy resources based on the release (default) variant from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-experimental
deploy-experimental: manifests-experimental $(KUSTOMIZE) ## Deploy resources based on the development variant to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${MANAGER_IMAGE}
	$(KUSTOMIZE) build config/development | kubectl apply -f -

.PHONY: undeploy-experimental
undeploy-experimental: $(KUSTOMIZE) ## Undeploy resources based on the development variant from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/development | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: update-metrics-docs
 update-metrics-docs: $(PROMLINTER) $(GOMPLATE) # Update metrics documentation
	@metrics=$$(mktemp).json; echo $${metrics}; $(PROMLINTER) list -ojson . > $${metrics}; $(GOMPLATE) -d telemetry=$${metrics} -f hack/telemetry-internal-metrics.md.tpl > docs/contributor/telemetry-internal-metrics.md
