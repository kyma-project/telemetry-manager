# E2E Test Local Execution Script

A convenient shell script for running e2e tests locally with dynamic cluster configuration.

## Location

`hack/run-e2e-test-local.sh`

## Quick Start

Run with default configuration (log-agent tests):
```bash
./hack/run-e2e-test-local.sh
```

## Common Usage Examples

### 1. Basic Log Agent Tests
```bash
./hack/run-e2e-test-local.sh
```

### 2. Tests with Istio
```bash
./hack/run-e2e-test-local.sh --istio -l "log-agent and istio"
```

### 3. Experimental Mode Tests
```bash
./hack/run-e2e-test-local.sh \
  --experimental \
  -p "./test/e2e/logs/gateway/..." \
  -l "log-gateway and experimental"
```

### 4. Custom Manager Image
```bash
./hack/run-e2e-test-local.sh \
  -i europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:pr-123
```

### 5. Metrics Agent Tests
```bash
./hack/run-e2e-test-local.sh \
  -p "./test/e2e/metrics/agent/..." \
  -l "metric-agent-a"
```

### 6. Trace Tests with Istio
```bash
./hack/run-e2e-test-local.sh \
  --istio \
  -p "./test/e2e/traces/..." \
  -l "traces and istio"
```

### 7. Custom Labels/Annotations (Non-FIPS)
```bash
./hack/run-e2e-test-local.sh \
  --no-fips \
  --custom-labels \
  -l "custom-label-annotation"
```

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-h, --help` | Show help message | - |
| `-i, --image IMAGE` | Manager image to use | `latest` |
| `--istio` | Install Istio before tests | `false` |
| `--no-fips` | Disable FIPS mode | FIPS enabled |
| `--experimental` | Enable experimental mode | `false` |
| `--custom-labels` | Enable custom labels/annotations | `false` |
| `-p, --path PATH` | Test path | `./test/e2e/logs/agent/...` |
| `-l, --labels LABELS` | Test labels filter | `log-agent` |
| `--no-verbose` | Disable verbose test output | Verbose enabled |
| `--skip-provision` | Skip cluster provisioning | Provision cluster |
| `--skip-cleanup` | Skip cleanup prompt | Show cleanup prompt |

## Environment Variables

You can also configure the script using environment variables:

```bash
export MANAGER_IMAGE="europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:pr-123"
export INSTALL_ISTIO="true"
export OPERATE_IN_FIPS_MODE="false"
export ENABLE_EXPERIMENTAL="true"
export TEST_PATH="./test/e2e/logs/gateway/..."
export TEST_LABELS="log-gateway and experimental"

./hack/run-e2e-test-local.sh
```

## What the Script Does

1. **Checks Prerequisites**: Verifies that required tools are installed (kubectl, k3d, docker, go, helm)
2. **Provisions K3D Cluster**: Creates a basic k3d cluster (or reuses existing)
3. **Sets Environment Variables**: Exports configuration as environment variables
4. **Runs Tests**: Executes the specified test suite with proper configuration
5. **Shows Cluster Info**: Displays cluster state before and after tests
6. **Cleanup Options**: Offers choices for cleanup (keep cluster, delete resources, or delete cluster)

## Cluster Reuse

The script is designed to support cluster reuse. After running tests, you can:

**Option 1: Keep cluster for next run** (recommended)
- Fastest option for running multiple test scenarios
- Cluster and Istio (if installed) remain ready

**Option 2: Delete only telemetry resources**
- Removes telemetry-manager and pipeline CRs
- Cluster and Istio remain for quick rerun
- Good for testing different manager configurations

**Option 3: Delete entire cluster**
- Full cleanup, starts fresh next time
- Use when switching between major scenarios

## Workflow Examples

### Running Multiple Test Scenarios

```bash
# Scenario 1: Basic log agent tests
./hack/run-e2e-test-local.sh
# Choose option 1 (keep cluster) at cleanup prompt

# Scenario 2: Add Istio and run integration tests
./hack/run-e2e-test-local.sh \
  --istio \
  --skip-provision \
  -p "./test/integration/..." \
  -l "istio"
# Choose option 2 (delete resources) at cleanup prompt

# Scenario 3: Test metrics with existing Istio
./hack/run-e2e-test-local.sh \
  --skip-provision \
  -p "./test/e2e/metrics/agent/..." \
  -l "metric-agent-a"
```

### Quick Development Loop

```bash
# 1. Provision once
./hack/run-e2e-test-local.sh --skip-cleanup

# 2. Make code changes

# 3. Build new image
make docker-build IMG=localhost:5000/telemetry-manager:dev
make docker-push IMG=localhost:5000/telemetry-manager:dev

# 4. Rerun tests with new image
MANAGER_IMAGE=localhost:5000/telemetry-manager:dev \
./hack/run-e2e-test-local.sh --skip-provision --skip-cleanup

# 5. Repeat steps 2-4 as needed
```

## Troubleshooting

### Script fails with "k3d not found"
Install k3d:
```bash
# macOS
brew install k3d

# Linux
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
```

### Script fails with "Docker daemon not running"
Start Docker Desktop or Docker daemon:
```bash
# macOS
open -a Docker

# Linux (systemd)
sudo systemctl start docker
```

### Tests fail due to old cluster state
Delete and recreate the cluster:
```bash
make delete-k3d
./hack/run-e2e-test-local.sh
```

Or let the script prompt you:
```bash
./hack/run-e2e-test-local.sh
# Answer 'y' when asked to delete existing cluster
```

### Istio installation times out
Check cluster resources:
```bash
kubectl get nodes
kubectl top nodes
kubectl -n istio-system get pods
```

Increase cluster resources in `hack/make/k3d.mk` if needed.

## Prerequisites

The script checks for these required tools:
- `kubectl` - Kubernetes CLI
- `k3d` - k3d cluster manager
- `docker` - Container runtime
- `go` - Go programming language
- `helm` - Helm package manager

## See Also

- [E2E Dynamic Cluster Configuration](../../docs/contributor/e2e-dynamic-cluster-config.md) - Detailed documentation
- [Testing Strategy](../../docs/contributor/testing.md) - Overall testing approach
- `test/testkit/kubeprep/` - Cluster preparation package implementation
