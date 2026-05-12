Format your response using the following structure. Include only the sections listed here, in this exact order. You can omit sections marked as "(optional)" when they are not applicable.

## What Changed
One or two sentences describing the change at a high level.

## Affected Signal Types
(optional) List which pipeline types are affected: logs, metrics, traces, or all. Omit this section entirely if the change is infrastructure (build, CI, dependencies), tooling, or operator-level with no signal-specific impact.


## Key Changes
- **`package/or/file`**: Description of what changed and why.
- **`package/or/file`**: Description of what changed and why.

## Breaking Changes
(optional) Any breaking changes to CRD fields, API types, or behavior.

## Notes for Reviewers
Highlight anything non-obvious: tricky logic, deliberate trade-offs, areas that need extra scrutiny, or follow-up issues.

## Release Notes Input
Describe user-facing changes, or write "None" if there are none.

**Required Action:** Actions users must take (omit if none).

**Recommended Action:** Optional actions users should consider (omit if none).

Plain description of the user-facing change itself.

Do not add any other sections, headings, or text outside of this structure.
Do not wrap your response in a code block.
Do not include a title or PR number at the top.

---

## Examples

### Example 1: Feature with Grouped Key Changes and Release Notes

## What Changed

Configures the `k8sattributes` processor with a node filter across all OTel Collector pipelines so each collector instance only watches Pods on its own Node instead of all Pods cluster-wide. On large clusters this reduces per-instance memory from ~300 MB to ~15–30 MB.

## Affected Signal Types

Logs, Metrics, Traces

## Key Changes

Processor Configuration:
- **`internal/otelcollector/config/common/processor_builders.go`**: Adds `filter.node_from_env_var: MY_NODE_NAME` to the `k8sattributes` processor builder so all collector types pick up the node filter.
- **`internal/otelcollector/config/common/types.go`**: Extends the `K8sAttributesConfig` struct with the new filter field.

Resource Generation:
- **`internal/resources/otelcollector/agent.go`**: Injects the `MY_NODE_NAME` environment variable (sourced from `spec.nodeName`) into each agent DaemonSet so the filter has a value at runtime.

Golden Files:
- **`internal/otelcollector/config/logagent/testdata/`**, **`internal/otelcollector/config/metricagent/testdata/`**, **`internal/otelcollector/config/otlpgateway/testdata/`**, **`internal/resources/otelcollector/testdata/`**: Updated to reflect the new processor and environment variable in all collector configurations.

## Notes for Reviewers

The node filter is only effective for DaemonSet deployments. The OTLP gateway was recently converted from a Deployment to a DaemonSet; applying the filter there was the primary motivation for this change. Any future reversion to a Deployment would make the filter a no-op rather than incorrect, so no guard is needed.

## Release Notes Input

**Recommended Action:** Review your VPA or resource limit settings for OTel Collector DaemonSets — limits sized for the old cluster-wide watch may now be over-provisioned.

Metrics, Traces, Logs: The `k8sattributes` processor now limits each collector instance's Kubernetes informer watch to Pods on its own Node. This reduces memory consumption significantly on large clusters (from ~300 MB to ~15–30 MB per instance on a 200-node cluster).


---

### Example 2: Small Fix with No Release Notes

## What Changed

Adds `NoExecute` and `NoSchedule` tolerations to the OTLP gateway DaemonSet so its Pods are scheduled on tainted Nodes, matching the existing behavior of the OTel agent and Fluent Bit DaemonSets.

## Key Changes

- **`internal/resources/otelcollector/otlp_gateway.go`**: Adds critical tolerations to the DaemonSet Pod spec.
- **`internal/resources/otelcollector/testdata/`**: Updates golden files to include the new tolerations in all OTLP gateway variants.

## Notes for Reviewers

The tolerations were already present on the metric and log agent DaemonSets; this was an oversight when the OTLP gateway was converted from a Deployment to a DaemonSet.

## Release Notes Input

None.
