# Istio Test Commands

Quick reference for running istio integration tests locally using the simplified `hack/run-e2e-test-local.sh` script.

## Basic Istio Test Command

Run all istio integration tests (equivalent to the old `integration-istio-otel` job):

```bash
./hack/run-e2e-test-local.sh --istio --build -p "./test/integration/istio/..." -l "istio"
```

## Specific Istio Test Configurations

### 1. OTEL with Istio (FIPS enabled)
Matches the old `integration-istio-otel` job:

```bash
./hack/run-e2e-test-local.sh --istio --build -p "./test/integration/istio/..." -l "istio and not fluent-bit"
```

### 2. Fluent-bit with Istio (FIPS disabled)
Matches the old `integration-istio-fluent-bit` job:

```bash
./hack/run-e2e-test-local.sh --istio --no-fips --build -p "./test/integration/istio/..." -l "istio and fluent-bit"
```

### 3. Experimental with Istio (FIPS enabled)
Matches the old `integration-istio-otel-experimental` job:

```bash
./hack/run-e2e-test-local.sh --istio --experimental --build -p "./test/integration/istio/..." -l "istio and experimental"
```

## Options Explained

- `--istio`: Installs Istio in the cluster before running tests
- `--build`: Builds a local telemetry-manager image with a timestamped tag
- `-p "./test/integration/istio/..."`: Runs all tests in the istio integration directory
- `-l "istio"`: Filters tests to only those labeled with `suite.LabelIstio`
- `--no-fips`: Disables FIPS mode (default is enabled)
- `--experimental`: Enables experimental features

## Reusing Cluster for Faster Iterations

If you already have a cluster provisioned and want to run tests again:

```bash
./hack/run-e2e-test-local.sh --skip-provision --istio -p "./test/integration/istio/..." -l "istio"
```

Or force recreate the cluster:

```bash
./hack/run-e2e-test-local.sh --force-recreate --istio --build -p "./test/integration/istio/..." -l "istio"
```

## Using a Custom Image

Instead of building locally, use a pre-built image:

```bash
./hack/run-e2e-test-local.sh --istio -i europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:pr-123 -p "./test/integration/istio/..." -l "istio"
```

## Test Label Combinations

The label system supports complex expressions:

```bash
# Run istio tests that are NOT fluent-bit and NOT experimental
-l "istio and not fluent-bit and not experimental"

# Run only fluent-bit istio tests
-l "istio and fluent-bit"

# Run only experimental istio tests
-l "istio and experimental"

# Run all istio tests (no filtering)
-l "istio"
```

## Cleanup

The script no longer has interactive cleanup prompts. TestMain handles cleanup automatically. If you need to manually clean up:

```bash
# Delete the cluster
k3d cluster delete kyma

# Or just delete telemetry resources (cluster reusable)
kubectl delete deployment telemetry-manager -n kyma-system --ignore-not-found
kubectl delete logpipelines --all
kubectl delete metricpipelines --all
kubectl delete tracepipelines --all
```
