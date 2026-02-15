# Example: Updating E2E Tests for Cluster Auto-Configuration

This file shows how to update existing e2e test suites to use the new cluster auto-configuration feature.

## Example 1: Basic Test Suite (No Cluster Config)

This is the simplest case - if your test doesn't need special cluster setup, no changes are needed:

```go
package agent

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	m.Run()
}
```

## Example 2: Test Suite with Automatic Cluster Setup

If you want to automatically configure the cluster before running tests:

```go
package agent

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
		log.Printf("Failed to load cluster config: %v", err)
		os.Exit(errorCode)
	}

	// Set the configuration so BeforeSuiteFunc uses it
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

## Example 3: Test Suite with Custom Logic

If you need more control over which tests get which cluster configuration:

```go
package agent

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	// Load manager image from environment
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		log.Printf("MANAGER_IMAGE environment variable is required")
		os.Exit(errorCode)
	}

	// Determine cluster configuration based on environment or test type
	cfg := &kubeprep.Config{
		ManagerImage:       managerImage,
		InstallIstio:       mustBool(os.Getenv("INSTALL_ISTIO"), false),
		OperateInFIPSMode:  mustBool(os.Getenv("OPERATE_IN_FIPS_MODE"), true),
		EnableExperimental: mustBool(os.Getenv("ENABLE_EXPERIMENTAL"), false),
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

func mustBool(s string, defaultVal bool) bool {
	switch s {
	case "true", "1", "yes":
		return true
	case "false", "0", "no", "":
		return defaultVal
	default:
		return defaultVal
	}
}
```

## Example 4: Conditional Setup Based on Test Labels

For more sophisticated scenarios where setup depends on which tests will run:

```go
package agent

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		log.Printf("MANAGER_IMAGE environment variable is required")
		os.Exit(errorCode)
	}

	// Check if we need Istio based on labels
	needsIstio := os.Getenv("LABELS") != "" && 
		contains(os.Getenv("LABELS"), "istio")
	
	// Check if we need experimental features
	needsExperimental := os.Getenv("LABELS") != "" &&
		contains(os.Getenv("LABELS"), "experimental")

	cfg := &kubeprep.Config{
		ManagerImage:       managerImage,
		InstallIstio:       needsIstio,
		OperateInFIPSMode:  true, // Always use FIPS in this suite
		EnableExperimental: needsExperimental,
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

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
```

## Migration Checklist

When updating test suites, follow this checklist:

- [ ] Import `kubeprep` package if cluster setup is needed
- [ ] Create or load `Config` based on environment/logic
- [ ] Set `suite.ClusterPrepConfig` before calling `BeforeSuiteFunc()`
- [ ] Add cleanup by calling `suite.AfterSuiteFunc()` after `m.Run()`
- [ ] Test with various environment variable combinations
- [ ] Verify cleanup happens even if tests fail (use defer or error handling)
- [ ] Update documentation or add comments explaining setup requirements

## Environment Variables in GitHub Actions

Update your GitHub Actions workflow to pass the necessary environment variables:

```yaml
- name: Execute tests
  uses: "./.github/template/test-execute"
  with:
    id: ${{ matrix.testcase.name }}
    path: ./test/${{ matrix.testcase.type }}/...
    labels: ${{ matrix.testcase.labels }}
  env:
    MANAGER_IMAGE: ${{ needs.setup.outputs.manager-image }}
    INSTALL_ISTIO: ${{ matrix.testcase.install-istio }}
    OPERATE_IN_FIPS_MODE: ${{ matrix.testcase.use-fips }}
    ENABLE_EXPERIMENTAL: ${{ matrix.testcase.mode == 'experimental' }}
```

## Troubleshooting Migration

### Issue: "MANAGER_IMAGE environment variable is required"

**Solution:** Make sure MANAGER_IMAGE is set before tests run:

```bash
export MANAGER_IMAGE=my-registry/telemetry-manager:v1.0
go test ./test/e2e/...
```

### Issue: Tests still using Makefile-based setup

**Solution:** Look for calls to `setup-e2e*` targets in the workflow and replace with environment variables.

### Issue: Cluster resources not cleaned up

**Solution:** Ensure `AfterSuiteFunc()` is called after tests complete:

```go
code := m.Run()
if err := suite.AfterSuiteFunc(); err != nil {
    log.Printf("Cleanup failed: %v", err)
}
os.Exit(code)
```

## Benefits of the New Approach

1. **Zero External Dependencies** - No Makefile or shell scripts needed
2. **Better Error Handling** - Go code provides better error context
3. **Easier Debugging** - Single process, easier to trace execution
4. **CI/CD Integration** - Direct environment variable support
5. **Testability** - Can unit test setup logic
6. **Maintainability** - All setup logic in Go with type safety

