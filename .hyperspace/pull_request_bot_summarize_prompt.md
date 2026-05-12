Summarize this pull request for the telemetry-manager project — a Kubernetes operator that manages telemetry pipelines (logs, metrics, traces) in Kyma clusters using OpenTelemetry Collectors and Fluent Bit.

Do not include a file-by-file list of changes. All change details belong in the "Key changes" section below.

Include all sections listed here, in this exact order. You can omit only sections marked as "(optional)" when they are not applicable.

## What Changed
One or two sentences describing the change at a high level.

## Affected Signal Types
(optional) List which pipeline types are affected: logs, metrics, traces, or all. Omit this section entirely if the change is infrastructure (build, CI, dependencies), tooling, or operator-level with no signal-specific impact.

## Key Changes
Bullet points covering the most important code changes. For each point, mention the relevant package or component (for example, `internal/reconciler/metricpipeline`, `internal/otelcollector/config`, `controllers/telemetry`).

## Breaking Changes
(optional) List any breaking changes to CRD fields, API types, or behavior. Omit this section if there are none.

## Notes for Reviewers
Highlight anything non-obvious: tricky logic, deliberate trade-offs, areas that need extra scrutiny, or follow-up issues.

## Release Notes Input
Always include this section. Describe any user-facing changes introduced by this PR for the release notes.

User-facing changes include, but are not limited to:
- New or changed pipeline behavior (logs, metrics, traces)
- New or changed CRD fields or API
- New or changed metrics, attributes, or resource enrichment
- Changes to OTel Collector or Fluent Bit configuration
- Deprecations or removals (deletions) of features or APIs
- Changes that require user action (migration steps, config updates)
- Changes that have a recommended (optional) user action (opt-in improvements, early adoption steps)
- RBAC or permission changes
- Service name or endpoint changes (breaks users hardcoding addresses)
- Workload kind changes (Deployment ↔ DaemonSet; affects HPA, PDB, external selectors)

If the change affects only specific signal types, start with the appropriate scope prefix:
- `Logs:` — log pipelines only
- `Metrics:` — metric pipelines only
- `Traces:` — trace pipelines only
- `Logs and Metrics:`, `Logs and Traces:`, `Metrics and Traces:` — two signal types
- No prefix — all signal types or non-signal-specific

If this PR has no user-facing changes (tests, docs, internal refactoring), write "None".

If this PR deprecates or removes a feature, describe what is deprecated or removed, why, and what users must migrate to.

If this PR introduces a feature where users should take an optional action (for example, enabling a new capability, adopting a new approach early, or updating dashboards), describe the recommended action and why it is beneficial.
