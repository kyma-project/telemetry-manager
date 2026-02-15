# Cluster Preparation Guide

This guide explains how to use the cluster preparation functionality to configure Kubernetes clusters for e2e testing, including Istio installation, FIPS mode configuration, and experimental feature deployment.

## Overview

The cluster preparation system allows you to:

1. **Install Istio** - Automatically deploy Istio in your cluster if needed
2. **Configure FIPS Mode** - Deploy telemetry components in FIPS 140-2 compliant mode
3. **Enable Experimental Features** - Deploy the telemetry manager with experimental features enabled
4. **Deploy Manager** - Automatically deploy the telemetry manager with the correct configuration
5. **Deploy Test Prerequisites** - Apply required test fixtures and configurations

## Usage in Test Suites

### Basic Setup (Manual Control)

If you don't need automatic cluster setup, simply use the existing setup:

```go
package mytest

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	// Optional: Set cluster preparation config here if desired
	// suite.ClusterPrepConfig = &kubeprep.Config{...}

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	code := m.Run()

	// Optional: Clean up cluster if setup was used
	// if err := suite.AfterSuiteFunc(); err != nil {
	//     log.Printf("Cleanup failed: %v", err)
	// }

	os.Exit(code)
}
```

### Automatic Cluster Setup from Environment Variables

If you want the test suite to automatically prepare the cluster based on environment variables:

```go
package mytest

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	// Load configuration from environment variables
	cfg, err := kubeprep.ConfigFromEnv()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		os.Exit(errorCode)
	}

	// Set the config so BeforeSuiteFunc uses it
	suite.ClusterPrepConfig = cfg

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	code := m.Run()

	// Clean up cluster after tests
	if err := suite.AfterSuiteFunc(); err != nil {
		log.Printf("Cleanup failed: %v", err)
	}

	os.Exit(code)
}
```

### Automatic Cluster Setup from Custom Logic

```go
package mytest

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	// Create config with custom values
	cfg := &kubeprep.Config{
		InstallIstio:       true,
		OperateInFIPSMode:  true,
		EnableExperimental: false,
		ManagerImage:       "my-registry.com/telemetry-manager:v1.0",
	}

	suite.ClusterPrepConfig = cfg

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	code := m.Run()

	if err := suite.AfterSuiteFunc(); err != nil {
		log.Printf("Cleanup failed: %v", err)
	}

	os.Exit(code)
}
```

## Environment Variables

When using `kubeprep.ConfigFromEnv()`, the following environment variables are supported:

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `MANAGER_IMAGE` | Container image for telemetry manager | Required | `europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:abc123` |
| `INSTALL_ISTIO` | Install Istio in the cluster | `false` | `true` |
| `OPERATE_IN_FIPS_MODE` | Deploy components in FIPS mode | `true` | `false` |
| `ENABLE_EXPERIMENTAL` | Enable experimental features | `false` | `true` |
| `CUSTOM_LABELS_ANNOTATIONS` | Add custom labels and annotations | `false` | `true` |

## Configuration Examples

### Release Mode with FIPS (Default)

```go
cfg := &kubeprep.Config{
	ManagerImage:       "my-image:latest",
	InstallIstio:       false,
	OperateInFIPSMode:  true,
	EnableExperimental: false,
}
```

Or via environment variables:

```bash
export MANAGER_IMAGE=my-image:latest
export INSTALL_ISTIO=false
export OPERATE_IN_FIPS_MODE=true
export ENABLE_EXPERIMENTAL=false
```

### Release Mode with Istio

```go
cfg := &kubeprep.Config{
	ManagerImage:       "my-image:latest",
	InstallIstio:       true,
	OperateInFIPSMode:  true,
	EnableExperimental: false,
}
```

### Experimental Mode without FIPS

```go
cfg := &kubeprep.Config{
	ManagerImage:       "my-image:latest",
	InstallIstio:       false,
	OperateInFIPSMode:  false,
	EnableExperimental: true,
}
```

### Experimental Mode with Istio and FIPS

```go
cfg := &kubeprep.Config{
	ManagerImage:       "my-image:latest",
	InstallIstio:       true,
	OperateInFIPSMode:  true,
	EnableExperimental: true,
}
```

## Integration with GitHub Actions

In your GitHub Actions workflow, you can now use environment variables to configure the cluster:

```yaml
- name: Run E2E Tests
  env:
    MANAGER_IMAGE: ${{ needs.setup.outputs.manager-image }}
    INSTALL_ISTIO: ${{ matrix.install-istio }}
    OPERATE_IN_FIPS_MODE: ${{ matrix.use-fips }}
    ENABLE_EXPERIMENTAL: ${{ matrix.mode == 'experimental' }}
  run: |
    go test ./test/e2e/... -labels="${{ matrix.labels }}"
```

## What Gets Deployed

When you run cluster preparation, the following happens:

1. **Namespace Creation** - Creates `kyma-system` namespace if it doesn't exist
2. **Istio Installation** (if `InstallIstio=true`):
   - Creates `istio-system` namespace
   - Installs Istio base using Helm
   - Installs Istiod using Helm
   - Applies Istio telemetry configuration
   - Applies peer authentication for mTLS
3. **Manager Deployment**:
   - Deploys telemetry manager via Helm with appropriate configuration
   - Sets FIPS mode based on configuration
   - Enables/disables experimental features based on configuration
   - Adds custom labels and annotations if requested
4. **Test Prerequisites**:
   - Applies `test/fixtures/operator_v1beta1_telemetry.yaml`
   - Applies `test/fixtures/networkpolicy-deny-all.yaml`
   - Applies `test/fixtures/shoot_info_cm.yaml`

## Cleanup

After tests complete, `AfterSuiteFunc()` will:

1. Remove the telemetry manager deployment
2. Clean up any resources created during setup

Note: Istio and test prerequisites are not automatically cleaned up, as they may be needed for other tests.

## Troubleshooting

### "MANAGER_IMAGE environment variable is required"

Make sure the `MANAGER_IMAGE` environment variable is set when using `ConfigFromEnv()`:

```bash
export MANAGER_IMAGE=your-registry/telemetry-manager:your-tag
```

### Cluster already has resources from previous tests

You can manually clean up by running:

```bash
kubectl delete -n kyma-system -f helm/charts/default/templates/
kubectl delete ns kyma-system
```

### Istio installation fails

Ensure you have `helm` installed and the Istio Helm repository is accessible:

```bash
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update
```

## Migration from Makefile-based Setup

**Old approach (via Makefile targets):**

```bash
make setup-e2e-istio
make deploy-experimental
# run tests
make cleanup-k3d
```

**New approach (via Go code):**

```bash
# Set up the cluster config in your test
suite.ClusterPrepConfig = &kubeprep.Config{
    InstallIstio:       true,
    EnableExperimental: true,
    ManagerImage:       os.Getenv("MANAGER_IMAGE"),
}
```

The advantages of the new approach:

1. **No external Makefile dependency** - Everything is in Go
2. **Better integration** - Cluster setup happens in the same process as tests
3. **Easier to customize** - Can use conditional logic based on test requirements
4. **Cleaner CI/CD** - Environment variables flow directly to code, no shell scripts

