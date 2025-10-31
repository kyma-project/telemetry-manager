##@ E2E Testing

# E2E test environment setup targets
.PHONY: setup-e2e
setup-e2e: provision-k3d wait-for-image deploy deploy-test-prerequisites ## Set up complete E2E test environment with k3d

.PHONY: setup-e2e-istio
setup-e2e-istio: provision-k3d-istio wait-for-image deploy deploy-test-prerequisites ## Set up E2E test environment with k3d and Istio

.PHONY: setup-e2e-experimental
setup-e2e-experimental: provision-k3d wait-for-image deploy-experimental deploy-test-prerequisites ## Set up E2E test environment with experimental features

.PHONY: setup-e2e-experimental-istio
setup-e2e-experimental-istio: provision-k3d-istio wait-for-image deploy-experimental deploy-test-prerequisites ## Set up E2E test environment with experimental features and Istio

.PHONY: deploy-test-prerequisites
deploy-test-prerequisites: ## Deploy common test prerequisites (telemetry config, network policy, shoot info)
	kubectl apply -f test/fixtures/operator_v1alpha1_telemetry.yaml -n kyma-system; \
	kubectl apply -f test/fixtures/networkpolicy-deny-all.yaml -n kyma-system; \
	kubectl apply -f test/fixtures/shoot_info_cm.yaml

# Default values for waiting for image
TIMEOUT ?= 900
QUERY_INTERVAL ?= 10
IMAGE_REPO ?= europe-docker.pkg.dev/kyma-project/dev/telemetry-manager

.PHONY: wait-for-image
wait-for-image: ## Wait for the manager image to be available in the registry
	@hack/await_image.sh


# Internal target for common e2e test execution logic
# Usage: $(call run-e2e-common,JUNIT_FLAGS)
define run-e2e-common
	echo "Running e2e tests with TEST_ID='$(TEST_ID)', TEST_PATH='$(TEST_PATH)', TEST_LABELS='$(TEST_LABELS)', ADDITIONAL LABELS='$(LABELS)'"
	@if [ -z "$(TEST_PATH)" ]; then \
		echo "Error: TEST_PATH environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$(TEST_LABELS)" ]; then \
		echo "Error: TEST_LABELS environment variable is required"; \
		exit 1; \
	fi
	@ALL_LABELS="$(TEST_LABELS)"; \
	if [ -n "$(LABELS)" ]; then \
		ADDITIONAL_LABELS=""; \
		for label in $(LABELS); do \
			if [ -z "$$ADDITIONAL_LABELS" ]; then \
				ADDITIONAL_LABELS="$$label"; \
			else \
				ADDITIONAL_LABELS="$$ADDITIONAL_LABELS,$$label"; \
			fi; \
		done; \
		ALL_LABELS="$$ALL_LABELS,$$ADDITIONAL_LABELS"; \
	fi; \
	echo "Using combined labels: $$ALL_LABELS"; \
	echo "Executing: $(GOTESTSUM) --format pkgname --hide-summary=skipped $(1) -- -timeout=20m $(TEST_PATH) -- -labels=\"$$ALL_LABELS\""; \
	$(GOTESTSUM) \
	--format pkgname \
	--hide-summary=skipped \
	$(1) \
	-- -timeout=20m $(TEST_PATH) \
	-- -labels="$$ALL_LABELS"
endef

.PHONY: run-e2e
run-e2e: $(GOTESTSUM) ## Run E2E tests (requires TEST_ID, TEST_PATH, and TEST_LABELS env vars)
	@if [ -z "$(TEST_ID)" ]; then \
		echo "Error: TEST_ID environment variable is required"; \
		exit 1; \
	fi
	$(call run-e2e-common,--junitfile junit-report-$(TEST_ID).xml)

.PHONY: run-e2e-no-junit
run-e2e-no-junit: $(GOTESTSUM) ## Run E2E tests without JUnit output
	$(call run-e2e-common,)

.PHONY: generate-e2e-targets
generate-e2e-targets: .github/workflows/pr-integration.yml ## Generate convenience targets for E2E tests from GitHub workflow matrix
	@echo '##@ E2E Test Suites' > hack/make/e2e-convenience.mk
	@echo '' >> hack/make/e2e-convenience.mk
	@cat .github/workflows/pr-integration.yml| yq -p yaml -o json | jq -r '.jobs.e2e.strategy.matrix.labels[]| ".PHONY: run-\(.type)-\(.name)\nrun-\(.type)-\(.name): ## Run \(.name) \(.type) tests\n\t$$(MAKE) run-e2e TEST_ID=\(.type)-\(.name) TEST_PATH=\"./test/\(.type)/...\" TEST_LABELS=\"\(.name)\"\n"' >> hack/make/e2e-convenience.mk

	@printf "\n.PHONY: run-all-e2e-logs\nrun-all-e2e-logs:" >> hack/make/e2e-convenience.mk
	@cat <(cat hack/make/e2e-convenience.mk | egrep '^run-e2e-(log|fluent)' | sed 's/:.*//') <(echo "## Run all log-related E2E tests") | xargs | sed 's/^/ /' >> hack/make/e2e-convenience.mk
	@echo >> hack/make/e2e-convenience.mk

	@printf ".PHONY: run-all-e2e-metrics\nrun-all-e2e-metrics:" >> hack/make/e2e-convenience.mk
	@cat <(cat hack/make/e2e-convenience.mk | egrep '^run-e2e-(metrics)' | sed 's/:.*//') <(echo "## Run all metrics-related E2E tests")| xargs | sed 's/^/ /' >> hack/make/e2e-convenience.mk
	@echo >> hack/make/e2e-convenience.mk

	@printf ".PHONY: run-all-e2e-traces\nrun-all-e2e-traces:" >> hack/make/e2e-convenience.mk
	@cat <(cat hack/make/e2e-convenience.mk | egrep '^run-e2e-(traces)' | sed 's/:.*//') <(echo "## Run all trace-related E2E tests") | xargs | sed 's/^/ /' >> hack/make/e2e-convenience.mk
	@echo >> hack/make/e2e-convenience.mk



-include hack/make/e2e-convenience.mk
