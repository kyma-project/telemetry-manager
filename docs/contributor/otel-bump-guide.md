# OpenTelemetry Dependency Bump Guide

Guide for maintainers and contributors when bumping `opentelemetry-collector` and `opentelemetry-collector-contrib` dependencies.

---

## Pre-bump Checklist

### 1. Review Changelog

Focus on these areas:
- **Filter Processor** changes
- **Transform Processor** changes
- **OTTL** (OpenTelemetry Transformation Language) updates
- **Internal metrics** modifications
- **Deprecation notices**

### 2. OTTL Changes

Check for:

#### New Functions
- Are there functions defined in the `filterprocessor` which are context specific, such as the [metric only functions](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor#ottl-functions)?
- **Action:** If new metric context-specific functions exist, disable them in filter processor and add a unit test for it, since we pin context to `datapoint` in MetricPipeline, metrics only functions will not be available for users.

#### Function Modifications
- Name changes
- Signature changes
- Function deprecations

### 3. Processor Updates

#### Filter Processor
- Review for context-specific function support requirements
- Check if any new functions need to be restricted (metric only functions)

#### Transform Processor
- Verify compatibility with existing transformations
- Test context switching behavior

### 4. Internal Metrics

Metrics can often change without notice, see

Common issues:
- Metric name changes
- Attribute additions/removals
- Metric type changes

## Post-Bump Verification

- [ ] All tests pass
- [ ] Run load test and document performance in [benchmark documentation](./benchmarks/results)
- [ ] Ensure running OTel components with FIPS mode enabled doesn't fail
- [ ] No new deprecation warnings (unless expected)
- [ ] Filter processor restrictions working correctly
