# OpenTelemetry Dependency Bump Guide

Guide for maintainers and contributors when bumping `opentelemetry-collector` and `opentelemetry-collector-contrib` dependencies.

---

## Pre-bump Checklist

### Review Changelog

Focus on these areas:

- Breaking changes, bug fixes, and enhancements for:
  - `kubeletstatsreceiver`
  - `k8sclusterreceiver`
  - `prometheusreceiver`
  - `otlpreceiver`
  - `otlpexporter`
  - `memorylimiter`
  - `k8sattributeprocessor`
  - `filterprocessor`
  - `transformprocessor`
  - `resourceprocessor`
- **OTTL** (OpenTelemetry Transformation Language) updates
- **Internal metrics** modifications
- **Deprecation notices**

### OTTL Changes

Check for:

#### Function Modifications
- Name changes
- Signature changes
- Function deprecations

> [!IMPORTANT]
> Processors may define additional OTTL functions which are restricted to specific contexts. The `filterprocessor` introduces [metrics only functions](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor#ottl-functions). If new metric context-specific functions exist, disable them in filter processor and add a unit test, since we pin context to `datapoint` in MetricPipeline, metrics only functions will not be available for users.

### Processor Updates

#### Filter Processor
- Monitor the availability of context inference in `filterprocessor` in this [issue](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/37904)

#### Transform Processor
- Verify any changes related to context inference

### Internal Metrics

Metrics can often change without notice. See examples below:

Common issues:
- [Prometheus metric name changes](https://github.com/open-telemetry/opentelemetry-collector/issues/13544)
- [Attribute additions/removals](https://github.com/open-telemetry/opentelemetry-collector/issues/9943)

---

## Post-Bump Verification

- [ ] All tests pass
- [ ] Run load test and document performance in [benchmark documentation](./benchmarks/results)
- [ ] Ensure running OTel components with FIPS mode enabled doesn't fail
- [ ] No new deprecation warnings (unless expected)
- [ ] Filter processor restrictions working correctly
