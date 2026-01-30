include .env

# Environment Variables
MANAGER_IMAGE ?= $(ENV_MANAGER_IMAGE)
FLUENT_BIT_EXPORTER_IMAGE ?= $(ENV_FLUENTBIT_EXPORTER_IMAGE)
FLUENT_BIT_IMAGE ?= $(ENV_FLUENTBIT_IMAGE)
OTEL_COLLECTOR_IMAGE ?= $(ENV_OTEL_COLLECTOR_IMAGE)
SELF_MONITOR_IMAGE ?= $(ENV_SELFMONITOR_IMAGE)
K3S_IMAGE ?= $(ENV_K3S_IMAGE)
ALPINE_IMAGE ?= $(ENV_ALPINE_IMAGE)
HELM_RELEASE_VERSION ?= $(ENV_HELM_RELEASE_VERSION)

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
CONVERSION_GEN   := $(TOOLS_BIN_DIR)/conversion-gen
GOLANGCI_LINT    := $(TOOLS_BIN_DIR)/golangci-lint
GO_TEST_COVERAGE := $(TOOLS_BIN_DIR)/go-test-coverage
GOTESTSUM        := $(TOOLS_BIN_DIR)/gotestsum
MOCKERY          := $(TOOLS_BIN_DIR)/mockery
TABLE_GEN        := $(TOOLS_BIN_DIR)/table-gen
YQ               := $(TOOLS_BIN_DIR)/yq
JQ               := $(TOOLS_BIN_DIR)/gojq
YAMLFMT          := $(TOOLS_BIN_DIR)/yamlfmt
STRINGER         := $(TOOLS_BIN_DIR)/stringer
K3D              := $(TOOLS_BIN_DIR)/k3d
PROMLINTER       := $(TOOLS_BIN_DIR)/promlinter
GOMPLATE         := $(TOOLS_BIN_DIR)/gomplate
HELM             := $(TOOLS_BIN_DIR)/helm
KUBECTL          := kubectl

POPULATE_IMAGES  := $(TOOLS_BIN_DIR)/populate-images

.PHONY: $(POPULATE_IMAGES)
$(POPULATE_IMAGES):
	cd $(DEPENDENCIES_DIR)/populateimages && go build -o $(POPULATE_IMAGES) main.go

# Sub-makefile
include hack/make/provision.mk
include hack/make/e2e.mk

##@ General

.PHONY: all
all: build ## Build the manager binary (default target)

.PHONY: help
help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Tools

.PHONY: install-tools
install-tools: $(TOOLS_BIN_NAMES) $(POPULATE_IMAGES) $(PROMLINTER) ## Install all required development tools

# Find dependency folders that contain go.mod
GO_MODULE_DIRS := $(shell find $(DEPENDENCIES_DIR) -mindepth 1 -maxdepth 1 -type d -exec test -f "{}/go.mod" \; -print)
MODULE_NAMES := $(notdir $(GO_MODULE_DIRS))

# All standard and fix lint targets
LINT_TARGETS := $(addprefix lint-,$(MODULE_NAMES))
LINT_FIX_TARGETS := $(addprefix lint-fix-,$(MODULE_NAMES))

# All build targets for dependencies
BUILD_DEPENDENCY_TARGETS := $(addprefix build-,$(MODULE_NAMES))

##@ Code Quality

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet --tags e2e,istio ./...

.PHONY: tidy
tidy: ## Check if there are any dirty changes for go mod tidy
	go mod tidy
	git diff --exit-code go.mod
	git diff --exit-code go.sum

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

.PHONY: lint
lint: lint-manager $(LINT_TARGETS) ## Run linting checks on all modules
	@echo "All lint checks completed."

.PHONY: lint-fix
lint-fix: lint-fix-manager $(LINT_FIX_TARGETS) ## Run linting checks with automatic fixes on all modules
	@echo "All lint fix checks completed."

.PHONY: lint-manager lint-fix-manager $(LINT_TARGETS) $(LINT_FIX_TARGETS)

##@ Code Generation

.PHONY: generate
generate: $(CONTROLLER_GEN) $(MOCKERY) $(STRINGER) $(YQ) $(YAMLFMT) $(POPULATE_IMAGES) $(CONVERSION_GEN) ## Generate code including DeepCopy, DeepCopyInto, DeepCopyObject methods and update helm values
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(CONVERSION_GEN) --go-header-file ./hack/boilerplate.go.txt --output-file zz_generated.conversion.go ./apis/telemetry/v1alpha1 ./apis/telemetry/v1beta1
	$(MOCKERY)
	$(STRINGER) --type Mode internal/utils/logpipeline/logpipeline.go
	$(STRINGER) --type FeatureFlag internal/featureflags/featureflags.go
	$(YQ) eval '.manager.container.env.fluentBitImage = ${FLUENT_BIT_IMAGE}' -i helm/values.yaml
	$(YQ) eval '.manager.container.env.fluentBitExporterImage = ${FLUENT_BIT_EXPORTER_IMAGE}' -i helm/values.yaml
	$(YQ) eval '.manager.container.env.otelCollectorImage = ${OTEL_COLLECTOR_IMAGE}' -i helm/values.yaml
	$(YQ) eval '.manager.container.env.selfMonitorImage = ${SELF_MONITOR_IMAGE}' -i helm/values.yaml
	$(YQ) eval '.manager.container.env.alpineImage = ${ALPINE_IMAGE}' -i helm/values.yaml
	$(YQ) eval '.manager.container.image.repository = "${MANAGER_IMAGE}"' -i helm/values.yaml
	$(YQ) eval '.version = "${HELM_RELEASE_VERSION}"' -i helm/Chart.yaml
	$(YQ) eval '.appVersion = "${HELM_RELEASE_VERSION}"' -i helm/Chart.yaml
	$(YQ) eval '.dependencies[].version = "${HELM_RELEASE_VERSION}"' -i helm/Chart.yaml
	$(YQ) eval '.version = "${HELM_RELEASE_VERSION}"' -i helm/charts/experimental/Chart.yaml
	$(YQ) eval '.version = "${HELM_RELEASE_VERSION}"' -i helm/charts/default/Chart.yaml
	$(YAMLFMT)
	$(POPULATE_IMAGES)

.PHONY: manifests
manifests: $(CONTROLLER_GEN) $(YQ) $(YAMLFMT) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition for v1alpha1 and v1beta1
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook crd paths="./..." output:crd:artifacts:config=helm/charts/default/templates
	$(YAMLFMT)

.PHONY: manifests-experimental
manifests-experimental: $(CONTROLLER_GEN) $(YAMLFMT) ## Generate manifests for experimental features (v1alpha1 and v1beta1)
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook crd paths="./..." output:crd:artifacts:config=helm/charts/experimental/templates
	$(YAMLFMT)

.PHONY: crd-docs-gen
crd-docs-gen: $(TABLE_GEN) manifests ## Generate CRD documentation in markdown format
	$(TABLE_GEN) --crd-filename ./helm/charts/default/templates/operator.kyma-project.io_telemetries.yaml --md-filename ./docs/user/resources/01-telemetry.md
	$(TABLE_GEN) --crd-filename ./helm/charts/default/templates/telemetry.kyma-project.io_logpipelines.yaml --md-filename ./docs/user/resources/02-logpipeline.md
	$(TABLE_GEN) --crd-filename ./helm/charts/default/templates/telemetry.kyma-project.io_tracepipelines.yaml --md-filename ./docs/user/resources/04-tracepipeline.md
	$(TABLE_GEN) --crd-filename ./helm/charts/default/templates/telemetry.kyma-project.io_metricpipelines.yaml --md-filename ./docs/user/resources/05-metricpipeline.md

.PHONY: check-clean
check-clean: generate manifests manifests-experimental crd-docs-gen generate-e2e-targets ## Check if repo is clean and up-to-date after code generation
	@echo "Checking if all generated files are up-to-date"
	@git diff --name-only --exit-code || (echo "Generated files are not up-to-date. Please run 'make generate manifests manifests-experimental crd-docs-gen generate-e2e-targets' to update them." && exit 1)

##@ Testing

.PHONY: test
test: manifests generate fmt vet tidy ## Run all unit tests
	go test ./test/testkit/matchers/...
	go test $$(go list ./... | grep -v /test/) -coverprofile cover.out
	cd ${PROJECT_DIR}/dependencies/directory-size-exporter && go test ./...
	cd ${PROJECT_DIR}/dependencies/sample-app && go test ./...

.PHONY: check-coverage
check-coverage: $(GO_TEST_COVERAGE) ## Check test coverage against thresholds
	go test $$(go list ./... | grep -v /test/) -short -coverprofile=cover.out -covermode=atomic -coverpkg=./...
	$(GO_TEST_COVERAGE) --config=./.testcoverage.yml

.PHONY: update-golden-files
update-golden-files: ## Update all golden files for config builder tests
	@echo "Updating all golden files in the project..."
	go test $$(go list ./... | grep -v /test/) -- -update-golden-files || true
	@echo "All golden files updated successfully"

##@ Build

.PHONY: build
build: build-manager fmt vet tidy ## Format, vet and build the manager

.PHONY: build-for-codeql
build-for-codeql: build-manager-no-generate build-dependencies ## Build the manager and the custom tools in dependencies

.PHONY: build-manager
build-manager: generate
	go build -o $(ARTIFACTS)/manager .

.PHONY: build-manager-no-generate
build-manager-no-generate:
	go build -o $(ARTIFACTS)/manager .

# Pattern rule for building each dependency module
$(BUILD_DEPENDENCY_TARGETS):
	@modname=$(@:build-%=%); \
	echo "Building $$modname..."; \
	cd $(DEPENDENCIES_DIR)/$$modname && pwd && go build -o $(ARTIFACTS)/$$modname .

.PHONY: build-dependencies $(BUILD_DEPENDENCY_TARGETS)
build-dependencies: $(BUILD_DEPENDENCY_TARGETS) ## Build custom tools in dependencies


.PHONY: docker-build
docker-build: ## Build docker image with the manager
	docker build -t ${MANAGER_IMAGE} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager
	docker push ${MANAGER_IMAGE}

.PHONY: docker-build-selfmonitor
docker-build-selfmonitor: ## Build docker image for telemetry self-monitor
	@set -a && . dependencies/telemetry-self-monitor/envs && set +a && \
	docker build -t ${SELF_MONITOR_IMAGE} \
		--build-arg ALPINE_VERSION=$${ALPINE_VERSION} \
		--build-arg PROMETHEUS_VERSION=$${PROMETHEUS_VERSION} \
		dependencies/telemetry-self-monitor

.PHONY: docker-push-selfmonitor
docker-push-selfmonitor: ## Push docker image for telemetry self-monitor
	docker push ${SELF_MONITOR_IMAGE}

##@ Development

.PHONY: run
run: gen-webhook-cert manifests generate fmt vet tidy ## Run controller from your host (requires webhook certificates)
	GODEBUG=fips140=only,tlsmlkem=0 go run ./main.go

# TLS certificate generation for local development
tls.key:
	@openssl genrsa -out tls.key 4096

tls.crt: tls.key
	@openssl req -sha256 -new -key tls.key -out tls.csr -subj '/CN=localhost'
	@openssl x509 -req -sha256 -days 3650 -in tls.csr -signkey tls.key -out tls.crt
	@rm tls.csr

.PHONY: gen-webhook-cert
gen-webhook-cert: tls.key tls.crt ## Generate TLS certificates for webhook development

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests $(HELM) ## Install CRDs into the K8s cluster
	$(HELM) template helm/charts/default | kubectl apply -f -

.PHONY: install-with-telemetry
install-with-telemetry: install ## Install CRDs and create sample telemetry resource
	kubectl get ns kyma-system || kubectl create ns kyma-system
	kubectl apply -f samples/operator_v1beta1_telemetry.yaml -n kyma-system

.PHONY: uninstall
uninstall: manifests $(HELM) ## Uninstall CRDs from the K8s cluster (use ignore-not-found=true to ignore missing resources)
	$(HELM) template helm/charts/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests $(HELM) ## Deploy telemetry manager with default/release configuration
	$(HELM) template telemetry helm \
		--set experimental.enabled=false \
		--set default.enabled=true \
		--set nameOverride=telemetry \
		--set manager.container.image.repository=${MANAGER_IMAGE} \
		--set manager.container.image.pullPolicy="Always" \
		--set manager.container.env.operateInFipsMode=true \
		--namespace kyma-system \
	| kubectl apply -f -

.PHONY: deploy-no-fips
deploy-no-fips: manifests $(HELM) ## Deploy telemetry manager with FIPS mode disabled
	$(HELM) template telemetry helm \
		--set experimental.enabled=false \
		--set default.enabled=true \
		--set nameOverride=telemetry \
		--set manager.container.image.repository=${MANAGER_IMAGE} \
		--set manager.container.image.pullPolicy="Always" \
		--set manager.container.env.operateInFipsMode=false \
		--namespace kyma-system \
	| kubectl apply -f -

.PHONY: undeploy
undeploy: $(HELM) ## Undeploy telemetry manager with default/release configuration
	$(HELM) template telemetry helm \
		--set experimental.enabled=false \
		--set default.enabled=true \
		--set nameOverride=telemetry \
		--namespace kyma-system \
	| kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-experimental
deploy-experimental: manifests-experimental $(HELM) ## Deploy telemetry manager with experimental features enabled
	$(HELM) template telemetry helm \
		--set experimental.enabled=true \
		--set default.enabled=false \
		--set nameOverride=telemetry \
		--set manager.container.image.repository=${MANAGER_IMAGE} \
		--set manager.container.image.pullPolicy="Always" \
		--set manager.container.env.operateInFipsMode=true \
		--set manager.container.args.deploy-otlp-gateway=true \
		--namespace kyma-system \
	| kubectl apply -f -

.PHONY: deploy-experimental-no-fips
deploy-experimental-no-fips: manifests-experimental $(HELM) ## Deploy telemetry manager with experimental features and FIPS mode disabled
	$(HELM) template telemetry helm \
		--set experimental.enabled=true \
		--set default.enabled=false \
		--set nameOverride=telemetry \
		--set manager.container.image.repository=${MANAGER_IMAGE} \
		--set manager.container.image.pullPolicy="Always" \
		--set manager.container.env.operateInFipsMode=false \
		--set manager.container.args.deploy-otlp-gateway=true \
		--namespace kyma-system \
	| kubectl apply -f -

.PHONY: deploy-custom-labels-annotations-no-fips
deploy-custom-labels-annotations-no-fips: manifests-experimental $(HELM) ## Deploy telemetry manager with experimental features, custom labels and annotations, and FIPS mode disabled
	$(HELM) template telemetry helm \
		--set experimental.enabled=true \
		--set default.enabled=false \
		--set nameOverride=telemetry \
		--set manager.container.image.repository=${MANAGER_IMAGE} \
		--set manager.container.image.pullPolicy="Always" \
		--set manager.container.env.operateInFipsMode=false \
		--set additionalMetadata.labels.my-meta-label="foo" \
		--set additionalMetadata.annotations.my-meta-annotation="bar" \
		--namespace kyma-system \
	| kubectl apply -f -

.PHONY: undeploy-experimental
undeploy-experimental: $(HELM) ## Undeploy telemetry manager with experimental features
	$(HELM) template telemetry helm \
		--set experimental.enabled=true \
		--set default.enabled=false \
		--set nameOverride=telemetry \
		--namespace kyma-system \
	| kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Documentation

.PHONY: update-metrics-docs
update-metrics-docs: $(PROMLINTER) $(GOMPLATE) ## Update internal metrics documentation
	@metrics=$$(mktemp).json; echo $${metrics}; $(PROMLINTER) list -ojson internal > $${metrics}; $(GOMPLATE) -d telemetry=$${metrics} -f hack/telemetry-internal-metrics.md.tpl > docs/contributor/telemetry-internal-metrics.md
