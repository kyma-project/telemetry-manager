# E2E Test Local Script - Quick Reference

## Basic Usage

```bash
# Run default tests (log-agent)
./hack/run-e2e-test-local.sh --build

# Run with existing cluster (faster iterations)
./hack/run-e2e-test-local.sh --skip-provision

# Force recreate cluster
./hack/run-e2e-test-local.sh --force-recreate --build
```

## Istio Tests (From Old E2E Matrix)

```bash
# All istio tests
./hack/run-e2e-test-local.sh --istio --build -p "./test/integration/istio/..." -l "istio"

# OTEL with Istio (old: integration-istio-otel)
./hack/run-e2e-test-local.sh --istio --build -p "./test/integration/istio/..." -l "istio and not fluent-bit"

# Fluent-bit with Istio (old: integration-istio-fluent-bit)
./hack/run-e2e-test-local.sh --istio --no-fips --build -p "./test/integration/istio/..." -l "istio and fluent-bit"

# Experimental (old: integration-istio-otel-experimental)
./hack/run-e2e-test-local.sh --istio --experimental --build -p "./test/integration/istio/..." -l "istio and experimental"
```

## Common Test Paths

```bash
# Logs
-p "./test/e2e/logs/agent/..." -l "log-agent"
-p "./test/e2e/logs/gateway/..." -l "log-gateway"
-p "./test/e2e/logs/fluentbit/..." -l "fluent-bit"

# Metrics
-p "./test/e2e/metrics/agent/..." -l "metric-agent-a"
-p "./test/e2e/metrics/gateway/..." -l "metric-gateway"

# Traces
-p "./test/e2e/traces/..." -l "traces"

# Integration
-p "./test/integration/istio/..." -l "istio"
```

## All Flags

| Flag | Description |
|------|-------------|
| `--build` | Build local manager image with timestamp |
| `--istio` | Install Istio in cluster |
| `--no-fips` | Disable FIPS mode (default: enabled) |
| `--experimental` | Enable experimental features |
| `--custom-labels` | Enable custom labels/annotations |
| `-p <path>` | Test path (e.g., `./test/e2e/logs/agent/...`) |
| `-l <labels>` | Label filter (e.g., `"log-agent and istio"`) |
| `-i <image>` | Use specific image instead of building |
| `--skip-provision` | Skip cluster provisioning |
| `--force-recreate` | Delete and recreate cluster |
| `--no-verbose` | Disable verbose test output |
| `--help` | Show help |

## Label Expressions

```bash
# Single label
-l "log-agent"

# AND operator
-l "log-agent and istio"

# NOT operator
-l "istio and not fluent-bit"

# Complex expression
-l "log-gateway and istio and not experimental"
```

## Using Custom Images

```bash
# Use PR image from registry
./hack/run-e2e-test-local.sh -i europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:pr-123 -l "log-agent"

# Use local built image (no --build flag)
./hack/run-e2e-test-local.sh -i telemetry-manager:my-tag --skip-provision
```

## Environment Variables

Can be set instead of flags:

```bash
export MANAGER_IMAGE="telemetry-manager:my-tag"
export INSTALL_ISTIO="true"
export OPERATE_IN_FIPS_MODE="false"
export TEST_PATH="./test/e2e/logs/agent/..."
export TEST_LABELS="log-agent"

./hack/run-e2e-test-local.sh
```

## Cleanup

Automatic via TestMain. Manual cleanup if needed:

```bash
# Delete cluster
k3d cluster delete kyma

# Delete only telemetry resources
kubectl delete deployment telemetry-manager -n kyma-system --ignore-not-found
kubectl delete logpipelines,metricpipelines,tracepipelines --all
```

## Script Changes (Old vs New)

| Feature | Old Script | New Script |
|---------|------------|------------|
| Line count | 448 lines | 258 lines |
| Interactive prompts | Yes (2) | No |
| Cleanup menu | Yes (4 options) | No (auto) |
| Cluster recreate | Interactive prompt | `--force-recreate` flag |
| Decorative output | Emojis, colors, boxes | Simple text |
| Prerequisites check | All tools | Docker only |
| Config display | Twice | Once |

See `E2E_SCRIPT_CLEANUP_SUMMARY.md` for detailed changes.
