##@ Build Dependencies
## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
K3D ?= $(LOCALBIN)/k3d
KYMA ?= $(LOCALBIN)/kyma-$(KYMA_STABILITY)

## Tool Versions
K3D_VERSION ?= $(ENV_K3D_VERSION)

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
