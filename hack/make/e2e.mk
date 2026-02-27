##@ E2E Testing

# Default values
TIMEOUT ?= 900
QUERY_INTERVAL ?= 10
IMAGE_REPO ?= europe-docker.pkg.dev/kyma-project/dev/telemetry-manager

# Test configuration (can be overridden)
E2E_TEST_LABELS ?=
E2E_TEST_PATH ?= ./test/e2e/...
E2E_TEST_ID ?= e2e
E2E_TEST_TIMEOUT ?= 60m
E2E_TEST_RUN ?=

# Use github-actions format when running in CI, otherwise use pkgname
GOTESTSUM_FORMAT ?= $(if $(GITHUB_ACTIONS),github-actions,pkgname)

.PHONY: wait-for-image
wait-for-image: ## Wait for the manager image to be available in the registry
	@hack/await_image.sh

# Main e2e test target
# Usage: make e2e-test E2E_TEST_PATH=./test/e2e/logs/... E2E_TEST_LABELS="logs"
# To run specific test function: make e2e-test E2E_TEST_PATH=./test/selfmonitor/... E2E_TEST_RUN="TestHealthy"
.PHONY: e2e-test
e2e-test: $(GOTESTSUM) ## Run E2E tests (use E2E_TEST_PATH, E2E_TEST_LABELS, E2E_TEST_ID, E2E_TEST_RUN)
	@echo "Running e2e tests..."
	@echo "  Path: $(E2E_TEST_PATH)"
	@echo "  Labels: $(E2E_TEST_LABELS)"
	@echo "  Test ID: $(E2E_TEST_ID)"
	@echo "  Timeout: $(E2E_TEST_TIMEOUT)"
	@echo "  Run filter: $(E2E_TEST_RUN)"
	$(GOTESTSUM) \
		--format $(GOTESTSUM_FORMAT) \
		--hide-summary=skipped \
		--junitfile junit-report-$(E2E_TEST_ID).xml \
		-- \
		-p 1 \
		-timeout $(E2E_TEST_TIMEOUT) \
		$(if $(E2E_TEST_RUN),-run "$(E2E_TEST_RUN)",) \
		$(E2E_TEST_PATH) \
		$(if $(E2E_TEST_LABELS),-args -labels="$(E2E_TEST_LABELS)",)

# Run e2e tests without JUnit output (for local development)
.PHONY: e2e-test-local
e2e-test-local: $(GOTESTSUM) ## Run E2E tests locally without JUnit output
	@echo "Running e2e tests locally..."
	@echo "  Path: $(E2E_TEST_PATH)"
	@echo "  Labels: $(E2E_TEST_LABELS)"
	@echo "  Run filter: $(E2E_TEST_RUN)"
	$(GOTESTSUM) \
		--format pkgname \
		--hide-summary=skipped \
		-- \
		-p 1 \
		-timeout $(E2E_TEST_TIMEOUT) \
		$(if $(E2E_TEST_RUN),-run "$(E2E_TEST_RUN)",) \
		$(E2E_TEST_PATH) \
		$(if $(E2E_TEST_LABELS),-args -labels="$(E2E_TEST_LABELS)",)

# Convenience targets for running tests by directory
.PHONY: e2e-logs
e2e-logs: ## Run logs E2E tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/e2e/logs/... E2E_TEST_ID=e2e-logs

.PHONY: e2e-metrics
e2e-metrics: ## Run metrics E2E tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/e2e/metrics/... E2E_TEST_ID=e2e-metrics

.PHONY: e2e-traces
e2e-traces: ## Run traces E2E tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/e2e/traces/... E2E_TEST_ID=e2e-traces

.PHONY: e2e-misc
e2e-misc: ## Run misc E2E tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/e2e/misc/... E2E_TEST_ID=e2e-misc

.PHONY: e2e-upgrade
e2e-upgrade: ## Run upgrade E2E tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/e2e/upgrade/... E2E_TEST_ID=e2e-upgrade

.PHONY: selfmonitor-test
selfmonitor-test: ## Run self-monitor tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/selfmonitor/... E2E_TEST_ID=selfmonitor

.PHONY: integration-test
integration-test: ## Run integration tests
	$(MAKE) e2e-test E2E_TEST_PATH=./test/integration/... E2E_TEST_ID=integration

# Legacy targets for backward compatibility
.PHONY: run-e2e
run-e2e: $(GOTESTSUM) ## [DEPRECATED] Run E2E tests (use e2e-test instead)
	@if [ -z "$(TEST_ID)" ]; then \
		echo "Error: TEST_ID environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$(TEST_PATH)" ]; then \
		echo "Error: TEST_PATH environment variable is required"; \
		exit 1; \
	fi
	$(MAKE) e2e-test \
		E2E_TEST_PATH="$(TEST_PATH)" \
		E2E_TEST_LABELS="$(TEST_LABELS)" \
		E2E_TEST_ID="$(TEST_ID)"
