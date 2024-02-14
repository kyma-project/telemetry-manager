MAKE_DEPS ?= hack/make
include ${MAKE_DEPS}/common.mk
include ${MAKE_DEPS}/test.mk


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




##@ TODO
# TODO: To be removed (unnecessary)
# test-matchers: ginkgo
# 	$(GINKGO) run ./test/testkit/matchers/...

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








# TODO: To GHA
.PHONY: release
release: kustomize ## Prepare release artefacts and create a GitHub release
	KUSTOMIZE=${KUSTOMIZE} IMG=${IMG} GORELEASER_VERSION=${GORELEASER_VERSION} ./hack/release.sh












# TODO: Use the ginkgo cli directly
GIT_COMMIT_DATE=$(shell git show -s --format=%cd --date=format:'v%Y%m%d' ${GIT_COMMIT_SHA})

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


