# OpenTelemetry Dependency Bump Guide

As a maintainer or contributor, follow these steps to update the `opentelemetry-collector` and `opentelemetry-collector-contrib` dependencies safely. Afterwards, verify the changes.

## Table of Content

- [OpenTelemetry Dependency Bump Guide](#opentelemetry-dependency-bump-guide)
  - [Table of Content](#table-of-content)
  - [Preparation](#preparation)
    - [1. Review Changed Components](#1-review-changed-components)
    - [2. Detect OTTL Changes](#2-detect-ottl-changes)
    - [3. Review Processor Updates](#3-review-processor-updates)
    - [4. Check Internal Metrics](#4-check-internal-metrics)
    - [5. Identify and Plan for Breaking Changes](#5-identify-and-plan-for-breaking-changes)
  - [Implementation](#implementation)
    - [OTel Collector Components](#otel-collector-components)
    - [Telemetry Manager](#telemetry-manager)
  - [Post-Bump Verification](#post-bump-verification)

## Preparation

Before you update the dependencies, review the changelog in the following repositories:

- [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector/releases)
- [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases)

 Focus on the following areas:

### 1. Review Changed Components

Identify breaking changes, bug fixes, and enhancements for the following components:
  - Receivers
    - `filelogreceiver` (contrib)
    - `kubeletstatsreceiver` (contrib)
    - `k8sclusterreceiver` (contrib)
    - `otlpreceiver`
    - `prometheusreceiver` (contrib)
  - Processors
    - `batchprocessor`
    - `filterprocessor` (contrib)
    - `k8sattributesprocessor` (contrib)
    - `memorylimiterprocessor`
    - `transformprocessor` (contrib)
  - Exporters
    - `otlpexporter`
    - `otlphttpexporter`
  - Extensions
    - `filestorage` (contrib)
    - `healthcheckextension` (contrib)
    - `k8sleaderelector` (contrib)
    - `pprofextension` (contrib)
  - Connectors
    - `forwardconnector`
    - `routingconnector` (contrib)
- **Deprecation notices**

### 2. Detect OTTL Changes

1. Check whether any functions changed.

   - New Functions
   - Name changes
   - Signature changes
   - Function deprecations

2. Check for incompatible OTTL function contexts.
   The `filterprocessor` may introduce functions that operate on entire metrics (using the `metric` context. However, our MetricPipeline operates on individual data points (a `datapoint` context) and cannot use such [metrics only functions](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor#ottl-functions).
   If you find new functions using the `metric` context, add them to the user documentation of functions that aren't supported by the filterprocessor.

### 3. Review Processor Updates

- Filter Processor: Monitor the availability of context inference in `filterprocessor` in this [issue](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/37904)

- Transform Processor: Verify any changes related to context inference

### 4. Check Internal Metrics

Metrics can often change without notice. See the following examples:

- [Prometheus metric name changes](https://github.com/open-telemetry/opentelemetry-collector/issues/13544)
- [Attribute additions/removals](https://github.com/open-telemetry/opentelemetry-collector/issues/9943)

### 5. Identify and Plan for Breaking Changes

Breaking changes are typically introduced behind feature gates, so you must check them:

1. Monitor feature gate lifecycles and track when feature gates are scheduled for removal.
2. Evaluate the impact of the change on our implementation.
3. If our code needs changes, plan to implement them before the feature gate is removed.


## Implementation

After you complete the preparation steps, update the dependency versions in the `opentelemetry-collector-components` and `telemetry-manager` repositories.

### OTel Collector Components

1. In the `opentelemetry-collector-components` repository, check out the OpenTelemetry update branch that Renovate automatically generates.
2. Update the dependency versions for `go.opentelemetry.io/collector` and `github.com/open-telemetry/opentelemetry-collector-contrib` in the following files:
   - `otel-collector/builder-config.yaml`
   - `otel-collector/envs`
   - `cmd/otelkymacol/builder-config.yaml`
3. Run `make gotidy`.
4. Run `make generate`.
5. Update and merge the PR.
6. After the image build GitHub Action completes, confirm that the new image tag is available.

### Telemetry Manager

1. In the `telemetry-manager` repository, update the dependency versions for `go.opentelemetry.io/collector` and `github.com/open-telemetry/opentelemetry-collector-contrib` in the following files:
   - `.env`
   - `go.mod`
   - `test/testkit/images.go`
   - `sec-scanners-config.yaml`
   - `helm/values.yaml`
2. Run `go mod tidy`.
3. Run `make generate`.
4. Create and merge a bump PR that references the merged `opentelemetry-collector-components` PR.

## Post-Bump Verification

After you updated the dependencies, perform the following verification checks:

- [ ] Verify that all tests pass.
- [ ] Manually trigger the "PR Load Test" GitHub workflow, and document the performance results of the load test in the [benchmark documentation](./benchmarks/results).
- [ ] Filter processor restrictions working correctly
