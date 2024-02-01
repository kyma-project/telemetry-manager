# 4. Consolidate Telemetry Pipeline Statuses

Date: 2024-01-02

## Status

Proposed

## Context

When we began the project, we lacked familiarity with the API conventions outlined in [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) about Custom Resource Definitions (CRD) statuses.
Instead, we made our own way by implementing a custom status condition structure, which was a subset of `metav1.Condition`.
Additionally, instead of treating status collections as a map with a key of type, we treated is as a dynamic list. At the start, a Pending condition was given to the resource upon creation. As underlying resources were deployed and ready, a Running condition was added. If issues arose, we cleared the entire list and only added a Pending condition. This deviates from API best practices and could be confusing for end-users.

That approach was used for `LogPipeline` and `TracePipeline` CRDs. Later on, we realized that we should follow the convention and use `metav1.Condition` instead. We did that for `Telemetry` and `MetricPipeline` CRDs.

### Goal

## Decision

## Consequences

