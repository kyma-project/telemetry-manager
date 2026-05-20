#!/usr/bin/env bash

# This script verifies that the local list of runtime additional metrics
# (KubeletStatsReceiverMetrics and K8sClusterReceiverMetrics) is in sync
# with the upstream OpenTelemetry Collector Contrib receiver definitions.

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

KUBELETSTATS_METRICS_FILE="${REPO_ROOT}/internal/otelcollector/config/metricagent/kubeletstats_metrics.go"
K8SCLUSTER_METRICS_FILE="${REPO_ROOT}/internal/otelcollector/config/metricagent/k8scluster_metrics.go"

# Step 1: Determine OTEL_CONTRIB_VERSION
echo "Fetching OTEL_CONTRIB_VERSION from kyma-project/opentelemetry-collector-components..."
ENVS_URL="https://raw.githubusercontent.com/kyma-project/opentelemetry-collector-components/main/otel-collector/envs"
OTEL_CONTRIB_VERSION=$(curl -sS "${ENVS_URL}" | grep "^OTEL_CONTRIB_VERSION=" | cut -d'=' -f2)

if [[ -z "${OTEL_CONTRIB_VERSION}" ]]; then
    echo "ERROR: Could not determine OTEL_CONTRIB_VERSION"
    exit 1
fi

echo "OTEL_CONTRIB_VERSION: ${OTEL_CONTRIB_VERSION}"
TAG="v${OTEL_CONTRIB_VERSION}"

# Step 2: Fetch upstream kubeletstatsreceiver metrics
echo ""
echo "Fetching upstream kubeletstatsreceiver metrics (${TAG})..."
KUBELETSTATS_METADATA_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector-contrib/${TAG}/receiver/kubeletstatsreceiver/metadata.yaml"
KUBELETSTATS_METADATA=$(curl -sS "${KUBELETSTATS_METADATA_URL}")

# Extract metric names from the metrics: section of the YAML
# Metrics are top-level keys under the "metrics:" section (indented with 2 spaces)
UPSTREAM_KUBELETSTATS_METRICS=$(echo "${KUBELETSTATS_METADATA}" | \
    sed -n '/^metrics:/,/^[a-z]/p' | \
    grep -E '^  [a-z]' | \
    sed 's/:.*//' | \
    sed 's/^  //' | \
    sort)

# Step 3: Fetch upstream k8sclusterreceiver metrics
echo "Fetching upstream k8sclusterreceiver metrics (${TAG})..."
K8SCLUSTER_METADATA_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector-contrib/${TAG}/receiver/k8sclusterreceiver/metadata.yaml"
K8SCLUSTER_METADATA=$(curl -sS "${K8SCLUSTER_METADATA_URL}")

# Extract metric names, excluding openshift-specific metrics
UPSTREAM_K8SCLUSTER_METRICS=$(echo "${K8SCLUSTER_METADATA}" | \
    sed -n '/^metrics:/,/^[a-z]/p' | \
    grep -E '^  [a-z]' | \
    sed 's/:.*//' | \
    sed 's/^  //' | \
    grep -v '^openshift\.' | \
    sort)

# Step 4: Extract local kubeletstats metrics from Go source
echo ""
echo "Extracting local KubeletStatsReceiverMetrics..."
LOCAL_KUBELETSTATS_METRICS=$(grep -oP '(?<= = ")[^"]+' "${KUBELETSTATS_METRICS_FILE}" | sort)

# Step 5: Extract local k8scluster metrics from Go source
echo "Extracting local K8sClusterReceiverMetrics..."
LOCAL_K8SCLUSTER_METRICS=$(grep -oP '(?<= = ")[^"]+' "${K8SCLUSTER_METRICS_FILE}" | sort)

# Step 6: Compare kubeletstats metrics
echo ""
echo "=== Comparing KubeletStatsReceiverMetrics ==="

MISSING_KUBELETSTATS=$(comm -23 <(echo "${UPSTREAM_KUBELETSTATS_METRICS}") <(echo "${LOCAL_KUBELETSTATS_METRICS}"))
EXTRA_KUBELETSTATS=$(comm -13 <(echo "${UPSTREAM_KUBELETSTATS_METRICS}") <(echo "${LOCAL_KUBELETSTATS_METRICS}"))

KUBELETSTATS_OK=true
if [[ -n "${MISSING_KUBELETSTATS}" ]]; then
    echo "ERROR: Local KubeletStatsReceiverMetrics is MISSING the following upstream metrics:"
    echo "${MISSING_KUBELETSTATS}" | sed 's/^/  - /'
    KUBELETSTATS_OK=false
fi

if [[ -n "${EXTRA_KUBELETSTATS}" ]]; then
    echo "ERROR: Local KubeletStatsReceiverMetrics has EXTRA metrics not found upstream:"
    echo "${EXTRA_KUBELETSTATS}" | sed 's/^/  - /'
    KUBELETSTATS_OK=false
fi

if [[ "${KUBELETSTATS_OK}" == "true" ]]; then
    echo "OK: KubeletStatsReceiverMetrics is in sync with upstream."
fi

# Step 7: Compare k8scluster metrics
echo ""
echo "=== Comparing K8sClusterReceiverMetrics ==="

MISSING_K8SCLUSTER=$(comm -23 <(echo "${UPSTREAM_K8SCLUSTER_METRICS}") <(echo "${LOCAL_K8SCLUSTER_METRICS}"))
EXTRA_K8SCLUSTER=$(comm -13 <(echo "${UPSTREAM_K8SCLUSTER_METRICS}") <(echo "${LOCAL_K8SCLUSTER_METRICS}"))

K8SCLUSTER_OK=true
if [[ -n "${MISSING_K8SCLUSTER}" ]]; then
    echo "ERROR: Local K8sClusterReceiverMetrics is MISSING the following upstream metrics:"
    echo "${MISSING_K8SCLUSTER}" | sed 's/^/  - /'
    K8SCLUSTER_OK=false
fi

if [[ -n "${EXTRA_K8SCLUSTER}" ]]; then
    echo "ERROR: Local K8sClusterReceiverMetrics has EXTRA metrics not found upstream:"
    echo "${EXTRA_K8SCLUSTER}" | sed 's/^/  - /'
    K8SCLUSTER_OK=false
fi

if [[ "${K8SCLUSTER_OK}" == "true" ]]; then
    echo "OK: K8sClusterReceiverMetrics is in sync with upstream."
fi

# Final result
echo ""
if [[ "${KUBELETSTATS_OK}" == "false" ]] || [[ "${K8SCLUSTER_OK}" == "false" ]]; then
    echo "FAILED: Local metrics are not in sync with upstream ${TAG}."
    echo "1. "
    exit 1
fi

echo "SUCCESS: All local metrics are in sync with upstream ${TAG}."
