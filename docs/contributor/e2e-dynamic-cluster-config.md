# E2E Test Dynamic Cluster Configuration

This document describes the dynamic cluster configuration system for e2e tests, which allows tests to automatically prepare a basic k3d cluster with the required configuration.

## Overview

Previously, e2e tests required pre-configured k3d clusters set up via Makefile targets (e.g., `setup-e2e-istio`, `setup-e2e-experimental`). This created complexity in CI/CD and local development workflows.

The new system allows tests to dynamically configure the cluster by:
1. Reading configuration from environment variables
2. Installing Istio (if needed)
3. Deploying telemetry-manager with the correct flags
4. Applying test fixtures
5. Automatically preparing the cluster before tests run

## Configuration

Tests are configured using environment variables:

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `MANAGER_IMAGE` | (required) | Telemetry manager container image to deploy |
| `INSTALL_ISTIO` | `false` | Whether to install Istio before tests |
| `OPERATE_IN_FIPS_MODE` | `true` | Deploy manager in FIPS 140-2 compliant mode |
| `ENABLE_EXPERIMENTAL` | `false` | Enable experimental CRDs |
| `CUSTOM_LABELS_ANNOTATIONS` | `false` | Add custom labels/annotations to components |
| `SKIP_MANAGER_DEPLOYMENT` | `false` | Skip manager deployment (used by upgrade tests) |
| `SKIP_PREREQUISITES` | `false` | Skip test prerequisites deployment |

## Local Development

### Basic Setup

1. Provision a basic k3d cluster:
   ```bash
   make provision-k3d
   ```

2. Set environment variables for your test scenario:
   ```bash
   export MANAGER_IMAGE=europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:pr-123
   export INSTALL_ISTIO=false
   export OPERATE_IN_FIPS_MODE=true
   export ENABLE_EXPERIMENTAL=false
   ```

3. Run tests:
   ```bash
   go test ./test/e2e/logs/agent/... -v -labels="log-agent"
   ```

The test will automatically configure the cluster based on the environment variables.

### Example Scenarios

**Scenario 1: Basic logs agent tests (no Istio, FIPS mode, standard)**
```bash
export MANAGER_IMAGE=your-image:tag
export INSTALL_ISTIO=false
export OPERATE_IN_FIPS_MODE=true
export ENABLE_EXPERIMENTAL=false
go test ./test/e2e/logs/agent/... -v -labels="log-agent"
```

**Scenario 2: Integration tests with Istio**
```bash
export MANAGER_IMAGE=your-image:tag
export INSTALL_ISTIO=true
export OPERATE_IN_FIPS_MODE=true
export ENABLE_EXPERIMENTAL=false
go test ./test/integration/... -v -labels="istio"
```

**Scenario 3: Experimental mode tests**
```bash
export MANAGER_IMAGE=your-image:tag
export INSTALL_ISTIO=false
export OPERATE_IN_FIPS_MODE=true
export ENABLE_EXPERIMENTAL=true
go test ./test/e2e/logs/gateway/... -v -labels="log-gateway and experimental"
```

**Scenario 4: Custom labels and annotations (non-FIPS mode)**
```bash
export MANAGER_IMAGE=your-image:tag
export INSTALL_ISTIO=false
export OPERATE_IN_FIPS_MODE=false
export ENABLE_EXPERIMENTAL=false
export CUSTOM_LABELS_ANNOTATIONS=true
go test ./test/e2e/... -v -labels="custom-label-annotation"
```

### Cluster Reuse

The cluster can be reused across multiple test runs with different configurations:

```bash
# Provision once
make provision-k3d

# Run multiple test scenarios without reprovisioning
export MANAGER_IMAGE=your-image:tag

# Test scenario 1
export INSTALL_ISTIO=false
go test ./test/e2e/logs/agent/... -v

# Test scenario 2 (Istio installed once, then reused)
export INSTALL_ISTIO=true
go test ./test/integration/... -v

# Test scenario 3 (reuses Istio from scenario 2)
export INSTALL_ISTIO=false  # Istio stays installed
go test ./test/e2e/traces/... -v
```

**Note:** Istio remains installed once deployed. To remove it, delete the cluster and reprovision.

### Backward Compatibility

If `MANAGER_IMAGE` is not set, tests will work with pre-configured clusters (existing behavior):

```bash
# Old approach still works
make setup-e2e
go test ./test/e2e/logs/agent/... -v
```

## CI/CD Integration

GitHub Actions workflows have been simplified to only provision a basic k3d cluster and set environment variables.

### Test Setup Action

The `.github/template/test-setup/action.yaml` now:
1. Provisions a basic k3d cluster (`make provision-k3d`)
2. Sets environment variables (`MANAGER_IMAGE`, `INSTALL_ISTIO`, etc.)
3. Does NOT call Makefile setup targets

### Test Execute Action

The `.github/template/test-execute/action.yaml` passes environment variables to test execution:
- `MANAGER_IMAGE`
- `INSTALL_ISTIO`
- `ENABLE_EXPERIMENTAL`
- `OPERATE_IN_FIPS_MODE`
- `CUSTOM_LABELS_ANNOTATIONS`

### Example Workflow

```yaml
- name: Test Setup
  uses: ./.github/template/test-setup
  with:
    manager-image: ${{ needs.build.outputs.image }}
    install-istio: "true"
    experimental: "false"
    operate-in-fips-mode: "true"

- name: Test Execute
  uses: ./.github/template/test-execute
  with:
    id: logs-agent-istio
    path: ./test/e2e/logs/agent/...
    labels: "log-agent and istio"
```

## Architecture

### Package Structure

```
test/testkit/kubeprep/
├── config.go        # Configuration and environment parsing
├── prepare.go       # Main cluster preparation orchestration
├── istio.go         # Istio installation
├── manager.go       # Telemetry manager deployment
├── prerequisites.go # Test fixtures deployment
├── cleanup.go       # Cluster cleanup utilities
└── utils.go         # Shared utilities
```

### Flow

```
┌──────────────────────────────────────────────┐
│  Environment Variables                        │
│  MANAGER_IMAGE, INSTALL_ISTIO, etc.          │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│  test/e2e/.../main_test.go                   │
│  func TestMain(m *testing.M) {               │
│      cfg := kubeprep.ConfigFromEnv()         │
│      suite.ClusterPrepConfig = cfg           │
│      suite.BeforeSuiteFunc()                 │
│  }                                            │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│  test/testkit/suite/suite.go                 │
│  func BeforeSuiteFunc() {                    │
│      if ClusterPrepConfig != nil {           │
│          kubeprep.PrepareCluster(cfg)        │
│      }                                        │
│      // Setup k8s clients                    │
│  }                                            │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│  test/testkit/kubeprep/prepare.go            │
│  func PrepareCluster(cfg) {                  │
│      1. Ensure kyma-system namespace         │
│      2. Install Istio (if cfg.InstallIstio) │
│      3. Deploy telemetry-manager             │
│      4. Deploy test prerequisites            │
│      5. Wait for readiness                   │
│  }                                            │
└──────────────────────────────────────────────┘
```

### Integration Points

Each test package's `main_test.go` includes cluster configuration loading:

```go
func TestMain(m *testing.M) {
    const errorCode = 1

    // Load cluster configuration from environment
    if managerImage := os.Getenv("MANAGER_IMAGE"); managerImage != "" {
        cfg, err := kubeprep.ConfigFromEnv()
        if err != nil {
            log.Printf("Failed to load cluster config: %v", err)
            os.Exit(errorCode)
        }

        suite.ClusterPrepConfig = cfg
    }

    if err := suite.BeforeSuiteFunc(); err != nil {
        log.Printf("Setup failed: %v", err)
        os.Exit(errorCode)
    }

    m.Run()
}
```

**Special case for upgrade tests:** `test/e2e/upgrade/main_test.go` sets `cfg.SkipManagerDeployment = true` because upgrade tests manage the deployment lifecycle themselves.

## Implementation Details

### Istio Installation

The `kubeprep.installIstio()` function translates `hack/deploy-istio.sh` to Go:
1. Reads Istio version from `.env` file (default: 2.11.0)
2. Applies istio-manager.yaml from GitHub release
3. Applies istio-default-cr.yaml from GitHub release
4. Waits for istiod deployment (10 retries × 30s)
5. Applies Istio Telemetry CR with access logging and tracing
6. Configures PeerAuthentication for strict mTLS in istio-system
7. Creates istio-permissive-mtls namespace with istio-injection label
8. Configures PeerAuthentication for permissive mTLS in istio-permissive-mtls

### Manager Deployment

The `kubeprep.deployManager()` function:
1. Runs `helm template` with configuration flags:
   - `experimental.enabled`
   - `manager.container.image.repository`
   - `manager.container.env.operateInFipsMode`
   - Custom labels/annotations (if enabled)
2. Applies generated YAML to cluster
3. Waits for telemetry-manager deployment to be ready

### Test Prerequisites

The `kubeprep.deployPrerequisites()` function applies:
1. Default Telemetry CR (`operator_v1beta1_telemetry.yaml`)
2. Network policy for kyma-system (`networkpolicy-deny-all.yaml`)
3. Shoot-info ConfigMap (`shoot_info_cm.yaml`)

These fixtures are embedded in the Go binary for reliability.

## Troubleshooting

### Test fails with "MANAGER_IMAGE is required"

Set the `MANAGER_IMAGE` environment variable:
```bash
export MANAGER_IMAGE=europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:latest
```

### Istio installation times out

Check if istiod deployment is running:
```bash
kubectl -n istio-system get deployment istiod
kubectl -n istio-system logs deployment/istiod
```

If needed, increase timeout in `test/testkit/kubeprep/istio.go:waitForIstiod()`.

### Manager deployment fails

Check telemetry-manager logs:
```bash
kubectl -n kyma-system get deployment telemetry-manager
kubectl -n kyma-system logs deployment/telemetry-manager
```

Verify helm chart is valid:
```bash
helm template telemetry ./helm --set manager.container.image.repository=your-image
```

### Tests fail due to stale cluster state

Clean up the cluster and reprovision:
```bash
make delete-k3d
make provision-k3d
```

Or manually clean up resources:
```bash
kubectl delete deployment telemetry-manager -n kyma-system
kubectl delete logpipelines --all
kubectl delete metricpipelines --all
kubectl delete tracepipelines --all
```

## Migration Guide

### For Test Developers

No changes needed! Tests automatically use the new system when `MANAGER_IMAGE` is set.

### For CI/CD Maintainers

Update workflows to:
1. Remove calls to `make setup-e2e*` targets
2. Add environment variable configuration in test-setup action
3. Ensure environment variables are passed to test-execute action

See updated `.github/template/test-setup/action.yaml` and `.github/template/test-execute/action.yaml`.

### For Local Development

Instead of:
```bash
make setup-e2e-istio
go test ./test/e2e/...
```

Use:
```bash
make provision-k3d
export MANAGER_IMAGE=your-image:tag
export INSTALL_ISTIO=true
go test ./test/e2e/...
```

## Benefits

1. **Simplified CI/CD**: No complex Makefile target orchestration
2. **Cluster reuse**: Run multiple test scenarios without reprovisioning
3. **Faster local development**: Just set env vars and run tests
4. **Better visibility**: Configuration is explicit in environment variables
5. **Easier maintenance**: Cluster setup logic centralized in Go code
6. **Backward compatible**: Works with pre-configured clusters when env vars not set

## Future Improvements

- Add cleanup between test runs (optional, currently cluster is reused)
- Support for additional configuration options (resource limits, node counts, etc.)
- Parallel test execution with cluster isolation
- Caching of Istio installation in CI runners
