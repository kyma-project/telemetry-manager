Summarize this pull request for the telemetry-manager project — a Kubernetes operator that manages telemetry pipelines (logs, metrics, traces) in Kyma clusters using OpenTelemetry Collectors and Fluent Bit.

Do not include a file-by-file list of changes. All change details belong in the "Key changes" section below.

Include all sections listed here, in this exact order. You can omit only sections marked as "(optional)" when they are not applicable.

## What Changed
One or two sentences describing the change at a high level.

## Affected Signal Types
(optional) List which pipeline types are affected: logs, metrics, traces, or all. Omit this section entirely if the change is infrastructure (build, CI, dependencies), tooling, or operator-level with no signal-specific impact.

## Key Changes
Bullet points covering the most important code changes. Format each bullet as **`path/to/file-or-package`**: description, where the path is the most relevant file, package, or component (for example, `internal/reconciler/metricpipeline`, `internal/otelcollector/config`, `controllers/telemetry`). Use the file path for small changes, the package path for larger changes spanning multiple files.

## Breaking Changes
(optional) List any breaking changes to CRD fields, API types, or behavior. Omit this section if there are none.

## Notes for Reviewers
Highlight anything non-obvious: tricky logic, deliberate trade-offs, areas that need extra scrutiny, or follow-up issues.

## Release Notes Input

Always include this section. If this PR has no user-facing changes (tests, docs, internal refactoring), write "None".

Structure the content using these three subsections, and include only the subsections that apply:

**Required Changes:** Actions users must take — migration steps, config updates, API changes, deprecation removals. Use this for anything that breaks existing behavior or requires user intervention. If this PR deprecates or removes a feature, describe what is deprecated or removed, why, and what users must migrate to.

**Recommended Changes:** Optional actions users should consider — enabling a new capability, updating dashboards, adopting a new approach early. Use this when the change is beneficial but users can continue without acting.

Followed by a plain description of the user-facing change itself (new behavior, new CRD fields, new metrics, etc.).

If this PR has no user-facing changes (tests, docs, internal refactoring), write "None" and omit all subsections.

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

If the change affects only specific signal types, prefix the section content with:
- `Logs:` — log pipelines only
- `Metrics:` — metric pipelines only
- `Traces:` — trace pipelines only
- `Logs and Metrics:`, `Logs and Traces:`, `Metrics and Traces:` — two signal types
- No prefix — all signal types or non-signal-specific
