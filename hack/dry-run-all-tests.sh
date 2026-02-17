#!/bin/bash
# This script simulates all matrix scenarios from pr-integration.yml
# using the -do-not-execute flag to show which tests would run.
#
# Output format (pipe-separated for easy parsing):
# testcasename | istio | experimental | fips

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

run_scenario() {
    local istio="$1"
    local experimental="$2"
    local fips="$3"
    local labels="$4"
    local path="$5"

    # Run go test with -do-not-execute flag and extract test names
    go test -v "$path" -count=1 -timeout=2m \
        -do-not-execute \
        -labels="$labels" \
        2>&1 | grep -E "^\[DRY-RUN\].*would execute" | \
        sed 's/\[DRY-RUN\] Test: \([^ ]*\).*/\1/' | \
        while read -r testname; do
            echo "$testname | $istio | $experimental | $fips"
        done
}

# Print header
echo "testcase | istio | experimental | fips"
echo "-------- | ----- | ------------ | ----"

# ==============================================================================
# E2E SELFMONITOR TESTS
# ==============================================================================

# fluent-bit scenarios (fips=no)
run_scenario "no" "no" "no" "selfmonitor-fluent-bit-healthy" "./test/selfmonitor/..."
run_scenario "yes" "no" "no" "selfmonitor-fluent-bit-backpressure" "./test/selfmonitor/..."
run_scenario "yes" "no" "no" "selfmonitor-fluent-bit-outage" "./test/selfmonitor/..."

# log-agent scenarios (fips=yes)
run_scenario "no" "no" "yes" "selfmonitor-log-agent-healthy" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-log-agent-backpressure" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-log-agent-outage" "./test/selfmonitor/..."

# log-gateway scenarios (fips=yes)
run_scenario "no" "no" "yes" "selfmonitor-log-gateway-healthy" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-log-gateway-backpressure" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-log-gateway-outage" "./test/selfmonitor/..."

# metric-gateway scenarios (fips=yes)
run_scenario "no" "no" "yes" "selfmonitor-metric-gateway-healthy" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-metric-gateway-backpressure" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-metric-gateway-outage" "./test/selfmonitor/..."

# metric-agent scenarios (fips=yes)
run_scenario "no" "no" "yes" "selfmonitor-metric-agent-healthy" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-metric-agent-backpressure" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-metric-agent-outage" "./test/selfmonitor/..."

# traces scenarios (fips=yes)
run_scenario "no" "no" "yes" "selfmonitor-traces-healthy" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-traces-backpressure" "./test/selfmonitor/..."
run_scenario "yes" "no" "yes" "selfmonitor-traces-outage" "./test/selfmonitor/..."

# ==============================================================================
# E2E CUSTOM LABELS ANNOTATIONS
# ==============================================================================

run_scenario "no" "no" "no" "custom-label-annotation" "./test/e2e/misc/..."

# ==============================================================================
# E2E TESTS - LOGS
# ==============================================================================

run_scenario "no" "no" "no" "fluent-bit and not experimental" "./test/e2e/..."
run_scenario "no" "no" "yes" "log-agent" "./test/e2e/..."
run_scenario "no" "no" "yes" "log-gateway and not experimental" "./test/e2e/..."
run_scenario "no" "yes" "yes" "log-gateway and experimental" "./test/e2e/..."
run_scenario "no" "no" "no" "logs-max-pipeline" "./test/e2e/..."
run_scenario "no" "no" "no" "fluent-bit-max-pipeline" "./test/e2e/..."
run_scenario "no" "no" "yes" "otel-max-pipeline" "./test/e2e/..."
run_scenario "no" "no" "yes" "logs-misc" "./test/e2e/..."

# ==============================================================================
# E2E TESTS - METRICS
# ==============================================================================

run_scenario "no" "no" "yes" "metric-agent-a" "./test/e2e/..."
run_scenario "no" "no" "yes" "metric-agent-b" "./test/e2e/..."
run_scenario "no" "no" "yes" "metric-agent-c" "./test/e2e/..."
run_scenario "no" "no" "yes" "metric-gateway-a" "./test/e2e/..."
run_scenario "no" "no" "yes" "metric-gateway-b" "./test/e2e/..."
run_scenario "no" "no" "yes" "metric-gateway-c" "./test/e2e/..."
run_scenario "no" "no" "yes" "metrics-misc" "./test/e2e/..."
run_scenario "no" "no" "yes" "metrics-max-pipeline" "./test/e2e/..."

# ==============================================================================
# E2E TESTS - TRACES
# ==============================================================================

run_scenario "no" "no" "yes" "traces" "./test/e2e/..."
run_scenario "no" "no" "yes" "traces-max-pipeline" "./test/e2e/..."

# ==============================================================================
# E2E TESTS - TELEMETRY
# ==============================================================================

run_scenario "no" "no" "yes" "telemetry and not fluent-bit" "./test/e2e/..."
run_scenario "no" "no" "no" "telemetry and fluent-bit" "./test/e2e/..."

# ==============================================================================
# E2E TESTS - MISC
# ==============================================================================

run_scenario "no" "no" "yes" "misc and not fluent-bit" "./test/e2e/..."
run_scenario "no" "no" "no" "misc and fluent-bit" "./test/e2e/..."

# ==============================================================================
# E2E TESTS - EXPERIMENTAL
# ==============================================================================

run_scenario "no" "yes" "yes" "experimental and not fluent-bit" "./test/e2e/..."
run_scenario "no" "yes" "no" "experimental and fluent-bit" "./test/e2e/..."

# ==============================================================================
# INTEGRATION TESTS - ISTIO
# ==============================================================================

run_scenario "yes" "no" "yes" "istio and not fluent-bit and not experimental" "./test/integration/..."
run_scenario "yes" "no" "no" "istio and fluent-bit" "./test/integration/..."
run_scenario "yes" "yes" "yes" "istio and experimental" "./test/integration/..."
