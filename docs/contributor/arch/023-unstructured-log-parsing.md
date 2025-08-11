---
title: Unstructured Log Parsing
status: Proposed
date: 2025-07-30
---
# 23. Unstructured Log Parsing

## Context

The current Fluent Bit-based LogPipeline custom resources offer minimal log parsing capabilities. However, with the ongoing migration toward OpenTelemetry (OTel)-based logging, we aim to maintain feature parity while improving the user experience and flexibility of configuration.

## Fluent Bit Log Parsing Overview

Fluent Bit supports various parsing mechanisms:
* Regex-based Parsing: Using the LogParser CR, applied to logs of specifically annotated workloads.
* Multiline Parsing: Through custom multiline filter.
* Built-in Parsers: For common log formats like JSON, Nginx, Apache, etc.

Limitations of the Fluent Bit Approach
* Requires manual annotation of each Pod to enable parsing.
* Limited support for mixed log formats within the same workload (needs further validation).
* Uses inconsistent APIs for different parsing types, which can confuse users.

## OTel Log Parsing: Requirements & Expectations

The OTel-based solution should fulfill the following requirements:

### Parser Definition
Because LogPipeline resources have output (backend) affinity and parsers have input (workload) affinity, parsers must not be defined in the LogPipeline. Ideally, they should be defined in a separate resource. However, for now, we want to avoid introducing a new API, and thus define parsers in the Telemetry CR.

### Workload Selection
Parsers must be tightly bound to a specific workload to ensure consistent application regardless of which pipeline is used.
It should be possible to bind parsers to workloads in a flexible way, allowing for:
 * Broad targeting with namespace selectors
 * Fine-grained targeting at the namespace → pod → container level
 * Optional label-based selectors for advanced use cases

### Parser Types
We should support two main types of parsers:
 * Regex-based parsers
 * Multiline parsers
These can be used independently or in sequence, depending on the log structure.

### Built-in Parser Presets
We should support presets for common formats (Java, Python, Nginx) to reduce duplication and improve UX.

# Decision

Here is a proposed API:
```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  log:
    parsers:
      - name: backend-parser
        namespaceSelector:
          name: backend-namespace
          # nameRegex: backend-namespace-.*
        podSelector:
          name: buttercup-app
          nameRegex: buttercup-app-.*
        containerSelector:
          name: server
          # nameRegex: server-.*
        multiline:
          type: builtin
          builtin: java
          # custom:
          #   firstEntryRegex: "^[^\\s]"
        regex:
          type: custom
          # builtin: nginx
          custom: "^Host=(?P<host>[^,]+), Type=(?P<type>.*)$"

```

## Tecnical Considerations

### Selector Support

Parsers should be applicable using the following options:

* Namespace/pod/container name or regex
* Pod/namespace label selectors (future enhancement)

To support label selectors, the Kubernetes metadata must be enriched before parsing. Thus, **parsing must happen after the `k8sattributes` processor**.

### Parser Implementation

#### Multiline Parsing

* Currently only feasible with the `recombine` operator in Stanza.
* Multiline parsing can be placed in one of the following receivers:

  * `filelog` receiver (limited flexibility)
  * `logtransform` receiver (more flexible, supports selectors, but currently alpha and not production-ready)

**Decision:** Defer label-selector-based parsing until `recombine` is available in the `transform` processor. For now, only support name and name regex-based selection. If many users request it, we may consider using the `logtransform` receiver as a temporary workaround.

#### Regex Parsing

* Implementable with one of the following options:

  * `regex_parser` operator (basic)
  * `transform` processor and OTTL expressions (preferred, more flexible and future-proof)

**Decision:** Use `transform` processor with OTTL for regex parsing.

### Reuse of Common Enrichment Logic

To ensure consistent enrichment across built-in and custom parsers:

* Reuse existing transformation logic (for example, extracting `message`, `level`, `traceparent`, etc.)
* Implement in `transform` processor using OTTL expressions.

## Action Plan

1. **Re-implement** the existing Stanza JSON parsing logic in the `transform` processor using OTTL.
2. **Implement multiline log parsing** using the `recombine` operator.
3. **Implement regex-based log parsing** using the `transform` processor.

## Example: Transform Processor Configuration

```yaml
transform:
  error_mode: ignore
  log_statements:
    # Try parse the body as JSON
    - conditions:
        - log.attributes["parsed"] == nil
      statements:
        - merge_maps(log.cache, ParseJSON(log.body), "upsert") where IsMatch(log.body, "^\\{")
        - merge_maps(log.attributes, log.cache, "upsert") where Len(log.cache) > 0
        - set(log.attributes["parsed"], true) where Len(log.cache) > 0

    # Try to parse the body as custom parser (python)
    - conditions:
        - log.attributes["parsed"] == nil
      statements:
        - merge_maps(log.attributes, ExtractPatterns(log.body, "File\\s+\"(?P<filepath>[^\"]+)\""), "upsert")
        - merge_maps(log.attributes, log.cache, "upsert") where Len(log.cache) > 0
        - set(log.attributes["parsed"], true) where Len(log.cache) > 0

    - conditions:
        - log.attributes["parsed"] != nil
      statements:
        - set(log.attributes["log.original"], log.body)
        - set(log.body, log.attributes["message"]) where log.attributes["message"] != nil
        - set(log.body, log.attributes["msg"]) where log.attributes["msg"] != nil
        - set(log.attributes["level"], log.attributes["log.level"]) where log.attributes["log.level"] != nil
        - set(log.severity_number, SEVERITY_NUMBER_INFO) where IsMatch(log.attributes["level"], "(?i)info")
        - set(log.severity_number, SEVERITY_NUMBER_WARN) where IsMatch(log.attributes["level"], "(?i)warn")
        - set(log.severity_number, SEVERITY_NUMBER_ERROR) where IsMatch(log.attributes["level"], "(?i)err")
        - set(log.severity_number, SEVERITY_NUMBER_DEBUG) where IsMatch(log.attributes["level"], "(?i)debug")
        - set(log.severity_text, ToUpperCase(log.attributes["level"])) where log.severity_number > 0
        - merge_maps(log.attributes, ExtractPatterns(log.attributes["traceparent"], "^(?P<trace_id>[0-9a-f]{32})-(?P<span_id>[0-9a-f]{16})-(?P<trace_flags>[0-9a-f]{2})$"), "upsert") where log.attributes["traceparent"] != nil
        - set(log.trace_id, log.attributes["trace_id"]) where log.attributes["trace_id"] != nil where log.attributes["traceparent"] == nil
        - set(log.span_id, log.attributes["span_id"]) where log.attributes["span_id"] != nil where log.attributes["traceparent"] == nil
        - set(log.flags, log.attributes["trace_flags"]) where log.attributes["trace_flags"] != nil where log.attributes["traceparent"] == nil
        - delete_matching_keys(log.attributes, "^(level|log.level|message|msg|parsed|span_id|trace_flags|trace_id|traceparent)$")
```

### Validation

Because we give users the flexibility to define custom parsers, we must ensure that custom parsers are well-formed and do not break the pipeline.
We can achieve this by:
* Validating parsers at the API level (for example, using a validation webhook).
* Reflect broken parsers in the Telemetry status, so that users can see which parsers are valid or broken.

**Decision**: Implement API-level validation for parsers using a validation webhook to give a user immediate feedback on parser correctness. Users using Kyma Dashboard will also see the status of parsers in the Telemetry resource.

## References

1. Built-in multiline parsers in Fluent Bit: https://github.com/fluent/fluent-bit/tree/master/src/multiline
