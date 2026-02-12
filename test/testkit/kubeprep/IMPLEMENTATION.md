# E2E Test Cluster Auto-Configuration Implementation

## Overview

This implementation provides a complete Go-based solution for configuring Kubernetes clusters for e2e testing. It replaces the previous approach where cluster configuration was handled via Makefile targets and GitHub Actions shell scripts.

## What Changed

### Before (Makefile-based)

```yaml
# GitHub Actions
- name: Setup test environment
  run: make setup-e2e-istio
- name: Deploy manager
  run: make deploy-experimental
- name: Deploy prerequisites
  run: make deploy-test-prerequisites
- name: Cleanup
  run: make cleanup-k3d
```

### After (Go code-based)

```go
// In your test's TestMain
suite.ClusterPrepConfig = &kubeprep.Config{
    InstallIstio:       true,
    EnableExperimental: true,
    ManagerImage:       os.Getenv("MANAGER_IMAGE"),
}

if err := suite.BeforeSuiteFunc(); err != nil {
    log.Fatalf("Setup failed: %v", err)
}

code := m.Run()

if err := suite.AfterSuiteFunc(); err != nil {
    log.Printf("Cleanup failed: %v", err)
}
```

## Architecture

### Package Structure

```
test/testkit/kubeprep/
├── kubeprep.go          # Core cluster preparation functionality
├── env.go               # Environment variable handling
├── README.md            # User guide
└── MIGRATION_GUIDE.md   # Migration examples

test/testkit/suite/
└── suite.go             # Updated with cluster prep integration
```

### Core Components

#### 1. `kubeprep.Config` - Cluster Configuration

Defines what should be deployed on the cluster:

```go
type Config struct {
    InstallIstio            bool
    OperateInFIPSMode       bool
    EnableExperimental      bool
    ManagerImage            string
    CustomLabelsAnnotations bool
}
```

#### 2. `PrepareCluster()` - Main Setup Function

Orchestrates the full cluster setup:

1. Ensures `kyma-system` namespace exists
2. Installs Istio (if configured)
3. Deploys telemetry manager (with proper flags)
4. Deploys test prerequisites

```go
func PrepareCluster(ctx context.Context, cfg Config) error
```

#### 3. `CleanupCluster()` - Teardown Function

Removes deployed resources after testing:

```go
func CleanupCluster(ctx context.Context, cfg Config) error
```

#### 4. Environment Variable Support

Configuration can be loaded from environment variables:

```go
cfg, err := kubeprep.ConfigFromEnv()  // Reads MANAGER_IMAGE, INSTALL_ISTIO, etc.
```

## Integration Points

### 1. Test Suite Integration

Updated `suite.BeforeSuiteFunc()` to call cluster preparation:

```go
// In test/testkit/suite/suite.go
var ClusterPrepConfig *kubeprep.Config  // Can be set by test suites

func BeforeSuiteFunc() error {
    // Prepare cluster if config is provided
    if ClusterPrepConfig != nil {
        if err := kubeprep.PrepareCluster(Ctx, *ClusterPrepConfig); err != nil {
            return err
        }
    }
    // ... rest of setup
}

func AfterSuiteFunc() error {
    // Cleanup cluster if config was used
    if ClusterPrepConfig != nil {
        return kubeprep.CleanupCluster(Ctx, *ClusterPrepConfig)
    }
}
```

### 2. GitHub Actions Integration

Instead of calling Makefile targets, tests pass environment variables:

```yaml
env:
  MANAGER_IMAGE: ${{ needs.setup.outputs.manager-image }}
  INSTALL_ISTIO: ${{ matrix.testcase.install-istio }}
  OPERATE_IN_FIPS_MODE: ${{ matrix.testcase.use-fips }}
  ENABLE_EXPERIMENTAL: ${{ matrix.testcase.mode == 'experimental' }}
```

## Supported Operations

### 1. Istio Installation

When `InstallIstio=true`:

- Creates `istio-system` namespace
- Adds Istio Helm repository
- Installs Istio base
- Installs Istiod
- Applies Istio telemetry configuration
- Applies peer authentication for mTLS

### 2. FIPS Mode Configuration

When `OperateInFIPSMode=true`:

- Passes `--set manager.container.env.operateInFipsMode=true` to Helm

### 3. Experimental Features

When `EnableExperimental=true`:

- Deploys experimental chart instead of default
- Sets `experimental.enabled=true`

### 4. Custom Labels and Annotations

When `CustomLabelsAnnotations=true`:

- Adds custom labels and annotations during Helm deployment

### 5. Test Prerequisites Deployment

Always deployed when cluster is prepared:

- `test/fixtures/operator_v1beta1_telemetry.yaml`
- `test/fixtures/networkpolicy-deny-all.yaml`
- `test/fixtures/shoot_info_cm.yaml`

## Benefits

### For Test Developers

1. **Cleaner TestMain** - No need for Makefile knowledge
2. **Type Safety** - Configuration is Go structs, not strings
3. **Better Errors** - Go errors provide clear context
4. **Flexibility** - Can use conditional logic for setup
5. **Documentation** - Configuration is self-documenting

### For CI/CD

1. **Simpler Workflows** - No shell script logic needed
2. **Environment Variables** - Configuration flows naturally
3. **Better Debugging** - Single process to trace
4. **Reduced Dependencies** - No Makefile required
5. **Consistency** - Same setup logic everywhere

### For Maintenance

1. **Single Source of Truth** - All setup logic in kubeprep package
2. **Easier Testing** - Setup logic can be unit tested
3. **Better Versioning** - Setup tied to code version
4. **Reduced Coupling** - No tight Makefile dependencies

## Usage Patterns

### Pattern 1: No Cluster Setup (Backward Compatible)

```go
// Don't set ClusterPrepConfig, use cluster as-is
if err := suite.BeforeSuiteFunc(); err != nil {
    log.Fatalf("Setup failed: %v", err)
}
```

### Pattern 2: Environment-Based Setup

```go
suite.ClusterPrepConfig, _ = kubeprep.ConfigFromEnv()
if err := suite.BeforeSuiteFunc(); err != nil {
    log.Fatalf("Setup failed: %v", err)
}
```

### Pattern 3: Custom Logic

```go
cfg := &kubeprep.Config{
    ManagerImage: os.Getenv("MANAGER_IMAGE"),
    InstallIstio: needsIstio(),  // Custom logic
}
suite.ClusterPrepConfig = cfg
if err := suite.BeforeSuiteFunc(); err != nil {
    log.Fatalf("Setup failed: %v", err)
}
```

## Migration Path

1. **Phase 1** - Infrastructure in place (current state)
   - New kubeprep package available
   - Existing tests continue to work
   - No breaking changes

2. **Phase 2** - Gradual adoption
   - Update test suites as needed
   - Maintain Makefile targets for backward compatibility
   - CI/CD workflows can use either approach

3. **Phase 3** - Full migration
   - All e2e tests use kubeprep
   - Makefile targets become optional
   - Simplified CI/CD workflows

## Files Created

1. **test/testkit/kubeprep/kubeprep.go** - Core functionality
2. **test/testkit/kubeprep/env.go** - Environment variable handling
3. **test/testkit/kubeprep/README.md** - User guide
4. **test/testkit/kubeprep/MIGRATION_GUIDE.md** - Migration examples
5. **test/testkit/suite/suite.go** - Updated with cluster prep integration

## Files Modified

1. **test/testkit/suite/suite.go** - Added:
   - `ClusterPrepConfig` variable
   - Cluster prep call in `BeforeSuiteFunc()`
   - New `AfterSuiteFunc()` for cleanup
   - Import of kubeprep package

## Next Steps

1. Update GitHub Actions workflow to pass environment variables
2. Gradually migrate e2e test suites (start with one test suite as pilot)
3. Remove Makefile setup targets once all suites are migrated
4. Update documentation and runbooks

## Examples

See `test/testkit/kubeprep/MIGRATION_GUIDE.md` for detailed examples of updating test suites.

## Troubleshooting

- **MANAGER_IMAGE not set**: Ensure environment variable is passed to tests
- **Istio installation fails**: Verify helm is installed and internet accessible
- **Cluster resources not cleaned**: Ensure AfterSuiteFunc() is called
- **Tests hanging**: Check kubectl is configured correctly for the cluster

## Future Enhancements

Potential improvements for future versions:

1. **Parallel Cluster Setup** - Prepare multiple clusters concurrently
2. **Cluster Validation** - Pre-flight checks before setup
3. **Metrics Collection** - Track setup time and resource usage
4. **Smart Cleanup** - Only clean up resources we created
5. **Retry Logic** - Automatic retries for transient failures
6. **Setup Caching** - Reuse cluster state between test runs

