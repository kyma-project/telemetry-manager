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
KUSTOMIZE_VERSION ?= $(ENV_KUSTOMIZE_VERSION)
TABLE_GEN_VERSION ?= $(ENV_TABLE_GEN_VERSION)
CONTROLLER_TOOLS_VERSION ?= $(ENV_CONTROLLER_TOOLS_VERSION)
K3D_VERSION ?= $(ENV_K3D_VERSION)
GINKGO_VERSION ?= $(ENV_GINKGO_VERSION)
GOLANGCI-LINT_VERSION ?= $(ENV_GOLANGCI-LINT_VERSION)
GO_TEST_COVERAGE_VERSION ?= $(ENV_GO_TEST_COVERAGE_VERSION)

.PHONY: dependencies
dependencies: kustomize tablegen controller-gen envtest golangci-lint ginkgo k3d kyma ## Download and install all build dependencies.

## kustomize
KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(KUSTOMIZE) && ! $(KUSTOMIZE) version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(KUSTOMIZE) version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(KUSTOMIZE); \
	fi
	test -s $(KUSTOMIZE) || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) --output install_kustomize.sh && bash install_kustomize.sh $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); rm install_kustomize.sh; }

## controller-gen
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

## envtest
.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(ENVTEST) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

## tablegen
.PHONY: tablegen
tablegen: $(TABLE_GEN) ## Download table-gen locally if necessary.
$(TABLE_GEN): $(LOCALBIN)
	test -s $(TABLE_GEN) || GOBIN=$(LOCALBIN) go install github.com/kyma-project/kyma/hack/table-gen@$(TABLE_GEN_VERSION)

## golangci-lint
.PHONY: golangci-lint
golangci-lint: $(GOLANGCI-LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI-LINT): $(LOCALBIN)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI-LINT_VERSION)

## ginkgo
.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(GINKGO) && $(GINKGO) version | grep -q $(GINKGO_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

## go-test-coverage
.PHONY: go-test-coverage
go-test-coverage: $(GO_TEST_COVERAGE) ## Download go-test-coverage locally if necessary.
$(GO_TEST_COVERAGE): $(LOCALBIN)
	test -s $(GO_TEST_COVERAGE) && $(GO_TEST_COVERAGE) --version | grep -q $(GO_TEST_COVERAGE_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/vladopajic/go-test-coverage/v2@$(GO_TEST_COVERAGE_VERSION)

## k3d
K3D_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh"
.PHONY: k3d
k3d: $(K3D) ## Download k3d locally if necessary. If wrong version is installed, it will be removed before downloading.
$(K3D): $(LOCALBIN)
	@if test -x $(K3D) && ! $(K3D) version | grep -q $(K3D_VERSION); then \
		echo "$(K3D) version is not as expected '$(K3D_VERSION)'. Removing it before installing."; \
		rm -rf $(K3D); \
	fi
	test -s $(K3D) || curl -s $(K3D_INSTALL_SCRIPT) | PATH="$(PATH):$(LOCALBIN)" USE_SUDO=false K3D_INSTALL_DIR=$(LOCALBIN) TAG=$(K3D_VERSION) bash

## Kyma
define os_error
$(error Error: unsupported platform OS_TYPE:$1, OS_ARCH:$2; to mitigate this problem set variable KYMA with absolute path to kyma-cli binary compatible with your operating system and architecture)
endef

KYMA_FILENAME ?=  $(shell hack/get-kyma-filename.sh ${OS_TYPE} ${OS_ARCH})
KYMA_STABILITY ?= unstable

.PHONY: kyma
kyma: $(LOCALBIN) $(KYMA) ## Download Kyma cli locally if necessary.
$(KYMA):
	$(if $(KYMA_FILENAME),,$(call os_error, ${OS_TYPE}, ${OS_ARCH}))
	test -f $@ || curl -s -Lo $(KYMA) https://storage.googleapis.com/kyma-cli-$(KYMA_STABILITY)/$(KYMA_FILENAME)
	chmod 0100 $(KYMA)
