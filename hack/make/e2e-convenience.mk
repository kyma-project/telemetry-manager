##@ E2E Test Suites

.PHONY: run-e2e-fluent-bit
run-e2e-fluent-bit: ## Run fluent-bit e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-fluent-bit TEST_PATH="./test/e2e/..." TEST_LABELS="fluent-bit"

.PHONY: run-e2e-log-agent
run-e2e-log-agent: ## Run log-agent e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-log-agent TEST_PATH="./test/e2e/..." TEST_LABELS="log-agent"

.PHONY: run-e2e-log-gateway
run-e2e-log-gateway: ## Run log-gateway e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-log-gateway TEST_PATH="./test/e2e/..." TEST_LABELS="log-gateway"

.PHONY: run-e2e-logs-max-pipeline
run-e2e-logs-max-pipeline: ## Run logs-max-pipeline e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-logs-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="logs-max-pipeline"

.PHONY: run-e2e-fluent-bit-max-pipeline
run-e2e-fluent-bit-max-pipeline: ## Run fluent-bit-max-pipeline e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-fluent-bit-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="fluent-bit-max-pipeline"

.PHONY: run-e2e-otel-max-pipeline
run-e2e-otel-max-pipeline: ## Run otel-max-pipeline e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-otel-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="otel-max-pipeline"

.PHONY: run-e2e-metric-agent-a
run-e2e-metric-agent-a: ## Run metric-agent-a e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metric-agent-a TEST_PATH="./test/e2e/..." TEST_LABELS="metric-agent-a"

.PHONY: run-e2e-metric-agent-b
run-e2e-metric-agent-b: ## Run metric-agent-b e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metric-agent-b TEST_PATH="./test/e2e/..." TEST_LABELS="metric-agent-b"

.PHONY: run-e2e-metric-agent-c
run-e2e-metric-agent-c: ## Run metric-agent-c e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metric-agent-c TEST_PATH="./test/e2e/..." TEST_LABELS="metric-agent-c"

.PHONY: run-e2e-metric-gateway-a
run-e2e-metric-gateway-a: ## Run metric-gateway-a e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metric-gateway-a TEST_PATH="./test/e2e/..." TEST_LABELS="metric-gateway-a"

.PHONY: run-e2e-metric-gateway-b
run-e2e-metric-gateway-b: ## Run metric-gateway-b e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metric-gateway-b TEST_PATH="./test/e2e/..." TEST_LABELS="metric-gateway-b"

.PHONY: run-e2e-metric-gateway-c
run-e2e-metric-gateway-c: ## Run metric-gateway-c e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metric-gateway-c TEST_PATH="./test/e2e/..." TEST_LABELS="metric-gateway-c"

.PHONY: run-e2e-metrics-misc
run-e2e-metrics-misc: ## Run metrics-misc e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metrics-misc TEST_PATH="./test/e2e/..." TEST_LABELS="metrics-misc"

.PHONY: run-e2e-metrics-max-pipeline
run-e2e-metrics-max-pipeline: ## Run metrics-max-pipeline e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-metrics-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="metrics-max-pipeline"

.PHONY: run-e2e-traces
run-e2e-traces: ## Run traces e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-traces TEST_PATH="./test/e2e/..." TEST_LABELS="traces"

.PHONY: run-e2e-traces-max-pipeline
run-e2e-traces-max-pipeline: ## Run traces-max-pipeline e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-traces-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="traces-max-pipeline"

.PHONY: run-e2e-telemetry
run-e2e-telemetry: ## Run telemetry e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-telemetry TEST_PATH="./test/e2e/..." TEST_LABELS="telemetry"

.PHONY: run-e2e-misc
run-e2e-misc: ## Run misc e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-misc TEST_PATH="./test/e2e/..." TEST_LABELS="misc"

.PHONY: run-e2e-experimental
run-e2e-experimental: ## Run experimental e2e tests
	$(MAKE) run-e2e TEST_ID=e2e-experimental TEST_PATH="./test/e2e/..." TEST_LABELS="experimental"

.PHONY: run-integration-istio
run-integration-istio: ## Run istio integration tests
	$(MAKE) run-e2e TEST_ID=integration-istio TEST_PATH="./test/integration/..." TEST_LABELS="istio"


.PHONY: run-all-e2e-logs
run-all-e2e-logs: run-e2e-fluent-bit run-e2e-log-agent run-e2e-log-gateway run-e2e-logs-max-pipeline run-e2e-fluent-bit-max-pipeline ## Run all log-related E2E tests

.PHONY: run-all-e2e-metrics
run-all-e2e-metrics: run-e2e-metrics-misc run-e2e-metrics-max-pipeline ## Run all metrics-related E2E tests

.PHONY: run-all-e2e-traces
run-all-e2e-traces: run-e2e-traces run-e2e-traces-max-pipeline ## Run all trace-related E2E tests

