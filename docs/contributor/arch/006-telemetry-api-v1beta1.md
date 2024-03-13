# 6. Telemetry API v1beta1

Date: 2024-01-02

## Status

Accepted

## Context

We have been using the `v1alpha1.telemetry.kyma-project.io` API group for a while now.
As changes have accumulated over time, making them in a compatible way is now impractical.
It's time to move to `v1beta1`.

## Decision

1. Create a new `v1beta1.telemetry.kyma-project.io` API group, mirroring `v1alpha1.telemetry.kyma-project.io` but with the following changes:

   * `Loki` output is removed from `LogPipeline`.
   * The `System` option in the LogPipeline input namespace selector is removed. Instead, it is replaced with the default behavior of excluding system namespaces, aligning with the approach taken for `MetricPipeline`.
   * `Application` input is renamed to `Tail` and an `Enabled` field is added to it, aligning with the approach taken for `MetricPipeline`.
   * `Running` and `Pending` conditions are removed from `LogPipeline` and `TracePipeline` (see [Consolidate Pipeline Statuses](./004-consolidate-pipeline-statuses.md)).
   * `LogParser` is removed.

2. Set `v1beta1` as the storage version in respective CRDs. Both `v1alpha1` and `v1beta1` will be served. Update the user-facing documentation.
3. Set up a conversion webhook to convert `v1alpha1` to `v1beta1` and vice versa. Since Loki output is not used anymore, a conversion with data loss is acceptable.
4. Change all API references in the codebase to `v1beta1`.
5. Run [storage version migrator](https://github.com/kubernetes-sigs/kube-storage-version-migrator) on customer clusters to migrate existing resources to `v1beta1`.
6. Announce the deprecation of `v1alpha1` in the Release Notes.
7. After the deprecation period, remove `v1alpha1` from the codebase, as well as from the served versions.

## Consequences

This marks our initial migration to a new API group. Ensuring the conversion webhook functions as expected and the storage version migrator doesn't cause issues is crucial.
