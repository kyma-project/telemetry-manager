# OpenTelemetry Dependency Bump Guide

Guide for maintainers and contributors when bumping `opentelemetry-collector` and `opentelemetry-collector-contrib` dependencies.

---

- [ ] Identify target version
- [ ] Review changelog for last n releases
- [ ] Check for breaking changes announcements
- [ ] Note any deprecation warnings

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
- Are there context-specific functions (metrics, datapoints, spans)?
- **Action:** If new metric context-specific functions exist, disable them in filter processor (we pin context to `datapoint` in MetricPipeline)

#### Function Modifications
- Name changes
- Signature changes
- Function deprecations

### 3. Processor Updates

#### Filter Processor
- Review for context-specific function support requirements
- Check if any new functions need to be restricted

#### Transform Processor
- Verify compatibility with existing transformations
- Test context switching behavior

### 4. Internal Metrics

Metrics can often change without notice, see

Common issues:
- Metric name changes
- Attribute additions/removals
- Metric type changes

Check: [OpenTelemetry Internal Telemetry Docs](https://opentelemetry.io/docs/collector/internal-telemetry/)

### 5. Testing

- [ ] Run E2E tests
- [ ] Check deprecation notice test (verify it passes)
- [ ] Validate internal metrics collection
- [ ] Test filter/transform processors with new version

---

## Known Issues Reference

### Issue: Missing `data_type` Attribute
**Problem:** `queue_size` and `queue_capacity` internal metrics missing data type attribute

- Issue: https://github.com/open-telemetry/opentelemetry-collector/issues/9943
- Fix: https://github.com/kyma-project/telemetry-manager/pull/1465/commits/1e629173d515f7cd8d75f48f6e274126bfd17e3f

### Issue: Metric Name Instability
**Problem:** `otelcol_exporter_send_failed_spans_total` renamed to `otelcol_exporter_send_failed_spans__spans__total` then reverted

- Issue: https://github.com/open-telemetry/opentelemetry-collector/issues/13544

**Takeaway:** Internal metric names are unstable. Always verify after bump.

---

## Post-Bump Verification

- [ ] All tests pass
- [ ] No new deprecation warnings (unless expected)
- [ ] Internal metrics align with expectations
- [ ] Filter processor restrictions working correctly
- [ ] Transform processor functions operational

---

## Quick Reference

**What breaks most often:**
1. Internal metrics (names, attributes, types)
2. OTTL function signatures
3. Context-specific function availability

**What to always check:**
1. Filter/Transform processor compatibility
2. OTTL function changes
3. Internal metrics stability
4. Deprecation notices
