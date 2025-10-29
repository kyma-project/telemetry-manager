.PHONY: setup-k8s-prerequisites
setup-k8s-prerequisites:
	kubectl apply -f samples/operator_v1alpha1_telemetry.yaml -n kyma-system; \
  kubectl apply -f samples/networkpolicy-deny-all.yaml -n kyma-system; \
  kubectl apply -f samples/shoot_info_cm.yaml


.PHONY: setup-e2e-istio setup-e2e
setup-e2e-istio: provision-k3d-istio deploy setup-k8s-prerequisites
setup-e2e: provision-k3d deploy setup-k8s-prerequisites


.PHONY: run-e2e
run-e2e: $(GOTESTSUM)
	@if [ -z "$(TEST_ID)" ]; then \
		echo "Error: TEST_ID environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$(TEST_PATH)" ]; then \
		echo "Error: TEST_PATH environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$(TEST_LABELS)" ]; then \
		echo "Error: TEST_LABELS environment variable is required"; \
		exit 1; \
	fi
	$(GOTESTSUM) \
	--format pkgname \
	--hide-summary=skipped \
	--junitfile junit-report-$(TEST_ID).xml \
	-- -timeout=20m $(TEST_PATH) \
	-- -labels="$(TEST_LABELS)" -labels=gardener

.PHONY: run-e2e-no-junit
run-e2e-no-junit: $(GOTESTSUM)
	@if [ -z "$(TEST_PATH)" ]; then \
		echo "Error: TEST_PATH environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$(TEST_LABELS)" ]; then \
		echo "Error: TEST_LABELS environment variable is required"; \
		exit 1; \
	fi
	$(GOTESTSUM) \
	--format pkgname \
	--hide-summary=skipped \
	-- -timeout=20m $(TEST_PATH) \
	-- -labels="$(TEST_LABELS)"

# Convenience targets for selfmonitor tests (matrix combinations)
.PHONY: run-e2e-selfmon-logs-fluentbit-healthy run-e2e-selfmon-logs-fluentbit-backpressure run-e2e-selfmon-logs-fluentbit-outage
run-e2e-selfmon-logs-fluentbit-healthy:
	$(MAKE) run-e2e TEST_ID=logs-fluentbit-healthy TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-fluentbit-healthy"

run-e2e-selfmon-logs-fluentbit-backpressure:
	$(MAKE) run-e2e TEST_ID=logs-fluentbit-backpressure TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-fluentbit-backpressure"

run-e2e-selfmon-logs-fluentbit-outage:
	$(MAKE) run-e2e TEST_ID=logs-fluentbit-outage TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-fluentbit-outage"

.PHONY: run-e2e-selfmon-logs-otel-agent-healthy run-e2e-selfmon-logs-otel-agent-backpressure run-e2e-selfmon-logs-otel-agent-outage
run-e2e-selfmon-logs-otel-agent-healthy:
	$(MAKE) run-e2e TEST_ID=logs-otel-agent-healthy TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-otel-agent-healthy"

run-e2e-selfmon-logs-otel-agent-backpressure:
	$(MAKE) run-e2e TEST_ID=logs-otel-agent-backpressure TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-otel-agent-backpressure"

run-e2e-selfmon-logs-otel-agent-outage:
	$(MAKE) run-e2e TEST_ID=logs-otel-agent-outage TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-otel-agent-outage"

.PHONY: run-e2e-selfmon-logs-otel-gateway-healthy run-e2e-selfmon-logs-otel-gateway-backpressure run-e2e-selfmon-logs-otel-gateway-outage
run-e2e-selfmon-logs-otel-gateway-healthy:
	$(MAKE) run-e2e TEST_ID=logs-otel-gateway-healthy TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-otel-gateway-healthy"

run-e2e-selfmon-logs-otel-gateway-backpressure:
	$(MAKE) run-e2e TEST_ID=logs-otel-gateway-backpressure TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-otel-gateway-backpressure"

run-e2e-selfmon-logs-otel-gateway-outage:
	$(MAKE) run-e2e TEST_ID=logs-otel-gateway-outage TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-logs-otel-gateway-outage"

.PHONY: run-e2e-selfmon-metrics-healthy run-e2e-selfmon-metrics-backpressure run-e2e-selfmon-metrics-outage
run-e2e-selfmon-metrics-healthy:
	$(MAKE) run-e2e TEST_ID=metrics-healthy TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-metrics-healthy"

run-e2e-selfmon-metrics-backpressure:
	$(MAKE) run-e2e TEST_ID=metrics-backpressure TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-metrics-backpressure"

run-e2e-selfmon-metrics-outage:
	$(MAKE) run-e2e TEST_ID=metrics-outage TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-metrics-outage"

.PHONY: run-e2e-selfmon-metrics-agent-healthy run-e2e-selfmon-metrics-agent-backpressure run-e2e-selfmon-metrics-agent-outage
run-e2e-selfmon-metrics-agent-healthy:
	$(MAKE) run-e2e TEST_ID=metrics-agent-healthy TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-metrics-agent-healthy"

run-e2e-selfmon-metrics-agent-backpressure:
	$(MAKE) run-e2e TEST_ID=metrics-agent-backpressure TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-metrics-agent-backpressure"

run-e2e-selfmon-metrics-agent-outage:
	$(MAKE) run-e2e TEST_ID=metrics-agent-outage TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-metrics-agent-outage"

.PHONY: run-e2e-selfmon-traces-healthy run-e2e-selfmon-traces-backpressure run-e2e-selfmon-traces-outage
run-e2e-selfmon-traces-healthy:
	$(MAKE) run-e2e TEST_ID=traces-healthy TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-traces-healthy"

run-e2e-selfmon-traces-backpressure:
	$(MAKE) run-e2e TEST_ID=traces-backpressure TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-traces-backpressure"

run-e2e-selfmon-traces-outage:
	$(MAKE) run-e2e TEST_ID=traces-outage TEST_PATH="./test/selfmonitor/..." TEST_LABELS="selfmonitor-traces-outage"

# Convenience targets for e2e tests
.PHONY: run-e2e-fluent-bit run-e2e-log-agent run-e2e-log-gateway run-e2e-logs-max-pipeline
run-e2e-fluent-bit:
	$(MAKE) run-e2e TEST_ID=e2e-fluent-bit TEST_PATH="./test/e2e/..." TEST_LABELS="fluent-bit"

run-e2e-log-agent:
	$(MAKE) run-e2e TEST_ID=e2e-log-agent TEST_PATH="./test/e2e/..." TEST_LABELS="log-agent"

run-e2e-log-gateway:
	$(MAKE) run-e2e TEST_ID=e2e-log-gateway TEST_PATH="./test/e2e/..." TEST_LABELS="log-gateway"

run-e2e-logs-max-pipeline:
	$(MAKE) run-e2e TEST_ID=e2e-logs-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="logs-max-pipeline"

.PHONY: run-e2e-fluent-bit-max-pipeline run-e2e-otel-max-pipeline
run-e2e-fluent-bit-max-pipeline:
	$(MAKE) run-e2e TEST_ID=e2e-fluent-bit-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="fluent-bit-max-pipeline"

run-e2e-otel-max-pipeline:
	$(MAKE) run-e2e TEST_ID=e2e-otel-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="otel-max-pipeline"

.PHONY: run-e2e-metric-agent-a run-e2e-metric-agent-b run-e2e-metric-agent-c
run-e2e-metric-agent-a:
	$(MAKE) run-e2e TEST_ID=e2e-metric-agent-a TEST_PATH="./test/e2e/..." TEST_LABELS="metric-agent-a"

run-e2e-metric-agent-b:
	$(MAKE) run-e2e TEST_ID=e2e-metric-agent-b TEST_PATH="./test/e2e/..." TEST_LABELS="metric-agent-b"

run-e2e-metric-agent-c:
	$(MAKE) run-e2e TEST_ID=e2e-metric-agent-c TEST_PATH="./test/e2e/..." TEST_LABELS="metric-agent-c"

.PHONY: run-e2e-metric-gateway-a run-e2e-metric-gateway-b run-e2e-metric-gateway-c
run-e2e-metric-gateway-a:
	$(MAKE) run-e2e TEST_ID=e2e-metric-gateway-a TEST_PATH="./test/e2e/..." TEST_LABELS="metric-gateway-a"

run-e2e-metric-gateway-b:
	$(MAKE) run-e2e TEST_ID=e2e-metric-gateway-b TEST_PATH="./test/e2e/..." TEST_LABELS="metric-gateway-b"

run-e2e-metric-gateway-c:
	$(MAKE) run-e2e TEST_ID=e2e-metric-gateway-c TEST_PATH="./test/e2e/..." TEST_LABELS="metric-gateway-c"

.PHONY: run-e2e-metrics-misc run-e2e-metrics-max-pipeline
run-e2e-metrics-misc:
	$(MAKE) run-e2e TEST_ID=e2e-metrics-misc TEST_PATH="./test/e2e/..." TEST_LABELS="metrics-misc"

run-e2e-metrics-max-pipeline:
	$(MAKE) run-e2e TEST_ID=e2e-metrics-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="metrics-max-pipeline"

.PHONY: run-e2e-traces run-e2e-traces-max-pipeline
run-e2e-traces:
	$(MAKE) run-e2e TEST_ID=e2e-traces TEST_PATH="./test/e2e/..." TEST_LABELS="traces"

run-e2e-traces-max-pipeline:
	$(MAKE) run-e2e TEST_ID=e2e-traces-max-pipeline TEST_PATH="./test/e2e/..." TEST_LABELS="traces-max-pipeline"

.PHONY: run-e2e-telemetry run-e2e-misc run-e2e-experimental
run-e2e-telemetry:
	$(MAKE) run-e2e TEST_ID=e2e-telemetry TEST_PATH="./test/e2e/..." TEST_LABELS="telemetry"

run-e2e-misc:
	$(MAKE) run-e2e TEST_ID=e2e-misc TEST_PATH="./test/e2e/..." TEST_LABELS="misc"

run-e2e-experimental:
	$(MAKE) run-e2e TEST_ID=e2e-experimental TEST_PATH="./test/e2e/..." TEST_LABELS="experimental"

# Integration tests
.PHONY: run-e2e-istio
run-e2e-istio:
	$(MAKE) run-e2e TEST_ID=integration-istio TEST_PATH="./test/integration/..." TEST_LABELS="istio"

# Grouped convenience targets
.PHONY: run-e2e-all-selfmon run-e2e-all-e2e run-e2e-all-logs run-e2e-all-metrics run-e2e-all-traces
run-e2e-all-selfmon: run-e2e-selfmon-logs-fluentbit-healthy run-e2e-selfmon-logs-fluentbit-backpressure run-e2e-selfmon-logs-fluentbit-outage run-e2e-selfmon-logs-otel-agent-healthy run-e2e-selfmon-logs-otel-agent-backpressure run-e2e-selfmon-logs-otel-agent-outage run-e2e-selfmon-logs-otel-gateway-healthy run-e2e-selfmon-logs-otel-gateway-backpressure run-e2e-selfmon-logs-otel-gateway-outage run-e2e-selfmon-metrics-healthy run-e2e-selfmon-metrics-backpressure run-e2e-selfmon-metrics-outage run-e2e-selfmon-metrics-agent-healthy run-e2e-selfmon-metrics-agent-backpressure run-e2e-selfmon-metrics-agent-outage run-e2e-selfmon-traces-healthy run-e2e-selfmon-traces-backpressure run-e2e-selfmon-traces-outage

run-e2e-all-e2e: run-e2e-fluent-bit run-e2e-log-agent run-e2e-log-gateway run-e2e-logs-max-pipeline run-e2e-fluent-bit-max-pipeline run-e2e-otel-max-pipeline run-e2e-metric-agent-a run-e2e-metric-agent-b run-e2e-metric-agent-c run-e2e-metric-gateway-a run-e2e-metric-gateway-b run-e2e-metric-gateway-c run-e2e-metrics-misc run-e2e-metrics-max-pipeline run-e2e-traces run-e2e-traces-max-pipeline run-e2e-telemetry run-e2e-misc run-e2e-experimental

run-e2e-all-logs: run-e2e-fluent-bit run-e2e-log-agent run-e2e-log-gateway run-e2e-logs-max-pipeline run-e2e-fluent-bit-max-pipeline run-e2e-otel-max-pipeline

run-e2e-all-metrics: run-e2e-metric-agent-a run-e2e-metric-agent-b run-e2e-metric-agent-c run-e2e-metric-gateway-a run-e2e-metric-gateway-b run-e2e-metric-gateway-c run-e2e-metrics-misc run-e2e-metrics-max-pipeline

run-e2e-all-traces: run-e2e-traces run-e2e-traces-max-pipeline

# Run all tests
.PHONY: run-e2e-all
run-e2e-all: run-e2e-all-selfmon run-e2e-all-e2e run-e2e-istio

