# 16a. Strategy for Supported OTTL Functions

Date: 25.7.2025

## Status

Accepted

## Context

OTTL functions enable users to customize their telemetry processing pipelines. While this extensibility brings significant benefits, it also introduces challenges related to performance, stability, and the lifecycle management of supported functions.

## Challenges

- **Stability:**  
  We currently lack experience with OTTL functions in a production scenario, so their stability is not yet proven.

- **Performance:**  
  The performance impact of user-defined OTTL configurations is unknown. Certain functions or combinations may have an considerable impact on performance.

- **Deprecations and Removals:**  
  With every release of otel-collector, functions may be deprecated or removed. While deprecated functions may continue to be used, we must guide our users for the upcoming removal of those functions.
  When a function is fully removed, pipelines still using this function become invalid and stop working.


## Decisions

1. **No Explicit Support List:**  
   Initially, we considered limiting support to a minimal subset of OTTL functions. This idea was discarded because it will lead to confusion if users refer to public documentation about OTTL.

2. **Transparent Communication:**  
   OTTL has reached [*beta* state](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#beta), so we do not anticipate *undocumented* breaking changes. As maintainers, we will monitor upstream updates and ensure that any additions, deprecations, or removals of OTTL functions are clearly communicated to our users through release notes and documentation.

4. **Deprecation Handling:**  
   As soon as a function is marked as deprecated in the upstream contribution, we will raise a warning in the status of every pipeline that uses such a function. 

5. **Removal Handling:**  
   Pipelines using removed functions are marked as erroneous and deactivated during reconciliation. The reason for deactivation must be set in the status of every pipeline that uses invalid functions.
