.PHONY: setup-k8s-prerequisites
setup-k8s-prerequisites:
	kubectl apply -f samples/operator_v1alpha1_telemetry.yaml -n kyma-system; \
  kubectl apply -f samples/networkpolicy-deny-all.yaml -n kyma-system; \
  kubectl apply -f samples/shoot_info_cm.yaml


.PHONY: setup-e2e-istio setup-e2e
setup-e2e-istio: provision-k3d-istio deploy setup-k8s-prerequisites
setup-e2e: provision-k3d deploy setup-k8s-prerequisites


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
run-e2e: $(GOTESTSUM)
	@if [ -z "$(TEST_ID)" ]; then \
		echo "Error: TEST_ID environment variable is required"; \
		exit 1; \
	fi
	$(call run-e2e-common,--junitfile junit-report-$(TEST_ID).xml)

.PHONY: run-e2e-no-junit
run-e2e-no-junit: $(GOTESTSUM)
	$(call run-e2e-common,)

.PHONY: generate-convenience
generate-convenience:
	@cat .github/workflows/pr-integration.yml| yq '.jobs.e2e.strategy.matrix.labels[]|".PHONY: run-\(.type)-\(.name)\nrun-\(.type)-\(.name):\n  $$(MAKE) run-e2e TEST_ID=\(.type)-\(.name) TEST_PATH=\"./test/\(.type)/...\" TEST_LABELS=\"\(.name)\"\n"' > hack/make/e2e-convenience.mk

	@printf ".PHONY: run-all-e2e-logs\nrun-all-e2e-logs: $$(cat hack/make/e2e-convenience.mk | egrep '^run-e2e-(log|fluent)' | tr -d ':' | xargs)\n" >> hack/make/e2e-convenience.mk
	@printf ".PHONY: run-all-e2e-metrics\nrun-all-e2e-metrics: $$(cat hack/make/e2e-convenience.mk | egrep '^run-e2e-(metrics)' | tr -d ':' | xargs)\n" >> hack/make/e2e-convenience.mk
	@printf ".PHONY: run-all-e2e-traces\nrun-all-e2e-traces: $$(cat hack/make/e2e-convenience.mk | egrep '^run-e2e-(traces)' | tr -d ':' | xargs)\n" >> hack/make/e2e-convenience.mk


-include hack/make/e2e-convenience.mk
