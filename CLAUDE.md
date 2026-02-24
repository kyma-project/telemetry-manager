# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Telemetry Manager is a Kubernetes operator (built with Kubebuilder) that manages telemetry pipelines for logs, traces, and metrics in Kyma clusters. It deploys and configures OpenTelemetry Collectors and Fluent Bit agents based on user-defined pipeline CRDs.

## Common Commands

### Build and Development
```bash
make build             # Format, vet, and build the manager binary
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
make e2e-upgrade       # Run upgrade E2E tests
make selfmonitor-test  # Run self-monitor tests
make integration-test  # Run integration tests
```

### Linting
```bash
make lint              # Run golangci-lint on all modules
make lint-fix          # Run linting with automatic fixes
```

### Cluster Setup
```bash
make provision-k3d        # Create k3d cluster with Kyma configuration
make provision-k3d-istio  # Create cluster and deploy Istio
make cleanup-k3d          # Delete k3d cluster

make install                # Install telemetry CRDs
make install-with-telemetry # Install CRDs and depoy default Telemetry CR
make uninstall              # Uninstall CRDs and webhook configurations
make deploy                 # Deploy telemetry manager with default configuration
make deploy-experimental    # Deploy with experimental features enabled
make undeploy               # Remove telemetry manager from cluster
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
- **test/integration/**: Integration tests with external components (e.g. Istio)
- **test/selfmonitor/**: Tests for self-monitoring functionality

### Helm Charts

- **helm/charts/default/**: Regular CRDs
- **helm/charts/experimental/**: Experimental CRDs
- **helm/**: Helm chart templates for deploying the manager (excluding charts subfolder which is documented above)

## Key Patterns

- Telemetry Manager uses the controller-runtime framework for building Kubernetes operators, with a focus on reconciliation loops that ensure the desired state of telemetry pipelines is maintained
- Each pipeline type has ist own controller and reconciler. Controller serves as a composition root for dependency injection and sets up watchers, while the reconciler contains the core business logic for managing resources related to that pipeline
- Configuration is built using builder patterns in `internal/*/config/` packages
- Dependent resources (Deployments, Services, ConfigMaps) are generated using `ApplierDeleters` in `internal/resources/`
- Mocks are generated with mockery (configured in `.mockery.yml`)
- E2E tests use go test with Gomega matchers (`test/testkit/suite/`)
- Golden file tests are used for config builders and resource appliers - update with `make update-golden-files`

## Environment Configuration

The `.env` file contains default image versions and configuration. Key environment variables for the manager are defined in `main.go` (envConfig struct).
