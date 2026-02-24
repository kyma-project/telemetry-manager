# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Telemetry Manager is a Kubernetes operator (built with Kubebuilder) that manages telemetry pipelines for logs, traces, and metrics in Kyma clusters. It deploys and configures OpenTelemetry Collectors and Fluent Bit agents based on user-defined pipeline CRDs.

## Common Commands

### Build and Development
```bash
make build              # Format, vet, and build the manager binary
make run               # Run controller locally (requires webhook certs, uses current kubeconfig)
make generate          # Generate DeepCopy methods, mocks, and update helm values
make manifests         # Generate CRDs, RBAC, and webhook manifests
make install-tools     # Install all required development tools to ./bin
```

### Testing
```bash
make test              # Run all unit tests
make check-coverage    # Run tests with coverage threshold checks

# E2E tests (require a cluster)
make e2e-logs          # Run logs E2E tests
make e2e-metrics       # Run metrics E2E tests
make e2e-traces        # Run traces E2E tests
make e2e-misc          # Run misc E2E tests
make selfmonitor-test  # Run self-monitor tests
make integration-test  # Run integration tests

# Run specific E2E test by function name
make e2e-test E2E_TEST_PATH=./test/e2e/logs/... E2E_TEST_RUN="TestSpecificFunction"
```

### Linting
```bash
make lint              # Run golangci-lint on all modules
make lint-fix          # Run linting with automatic fixes
```

### Cluster Setup
```bash
make provision-k3d     # Create k3d cluster with Kyma configuration
make provision-k3d-istio  # Create cluster and deploy Istio
make cleanup-k3d       # Delete k3d cluster

make deploy            # Deploy telemetry manager with default configuration
make deploy-experimental  # Deploy with experimental features enabled
make undeploy          # Remove telemetry manager from cluster
```

### Code Generation
```bash
make update-golden-files  # Update golden files for config builder tests
```

## Architecture

### Core Components

- **main.go**: Entry point that sets up the Kubernetes controller manager, registers schemes, and starts controllers
- **controllers/telemetry/**: Pipeline controllers that watch CRDs and trigger reconciliation
  - `logpipeline_controller.go` - LogPipeline CR controller
  - `metricpipeline_controller.go` - MetricPipeline CR controller
  - `tracepipeline_controller.go` - TracePipeline CR controller
- **controllers/operator/**: Telemetry module lifecycle controller

### Internal Packages

- **internal/reconciler/**: Business logic for each pipeline type
  - `logpipeline/fluentbit/` - Fluent Bit based log pipeline reconciler
  - `logpipeline/otel/` - OTel Collector based log pipeline reconciler
  - `metricpipeline/` - Metric pipeline reconciler
  - `tracepipeline/` - Trace pipeline reconciler
  - `telemetry/` - Module-level reconciler for overall Telemetry CR
- **internal/otelcollector/config/**: OTel Collector configuration builders
- **internal/fluentbit/config/**: Fluent Bit configuration builders
- **internal/resources/**: Kubernetes resource generators (deployments, services, etc.)
- **internal/selfmonitor/**: Prometheus-based health monitoring
- **internal/validators/**: Pipeline validation logic (TLS certs, endpoints, secrets)
- **internal/workloadstatus/**: Deployment/DaemonSet health checking

### API Versions

- **apis/telemetry/v1beta1/**: Current stable API (LogPipeline, MetricPipeline, TracePipeline)
- **apis/telemetry/v1alpha1/**: Legacy API with conversion webhooks to v1beta1
- **apis/operator/v1beta1/**: Telemetry module CR (top-level configuration)

### Test Structure

- **test/e2e/**: End-to-end tests organized by signal type (logs/, metrics/, traces/)
- **test/testkit/**: Shared test utilities, matchers, and k8s helpers
- **test/integration/**: Integration tests with external components

### Helm Charts

- **helm/charts/default/**: Production CRDs and manifests
- **helm/charts/experimental/**: Experimental features

## Key Patterns

- Pipeline reconcilers use the controller-runtime library pattern with `Reconcile()` methods
- Configuration is built using builder patterns in `internal/*/config/` packages
- Mocks are generated with mockery (configured in `.mockery.yml`)
- E2E tests use Ginkgo/Gomega with a custom test suite (`test/testkit/suite/`)
- Golden file tests are used for configuration builders - update with `make update-golden-files`

## Environment Configuration

The `.env` file contains default image versions and configuration. Key environment variables for the manager are defined in `main.go` (envConfig struct).
