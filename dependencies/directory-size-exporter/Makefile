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
	mkdir -p $@

$(TOOLS_BIN_NAMES): $(TOOLS_BIN_DIR) $(TOOLS_MOD_DIR)/go.mod
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES_CLEAN))%,$(TOOLS_PKG_NAMES))

## Tools
GOLANGCI_LINT    := $(TOOLS_BIN_DIR)/golangci-lint
WSL              := $(TOOLS_BIN_DIR)/wsl

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

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

resolve:
	GO111MODULE=on go mod vendor -v

mod-verify:
	GO111MODULE=on go mod verify

test: fmt vet tidy
	go test ./... -coverprofile cover.out

build: fmt vet tidy
	go build -o bin/exporter main.go

run: fmt vet tidy
	go run ./main.go
