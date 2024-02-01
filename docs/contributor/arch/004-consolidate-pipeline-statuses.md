# 4. Consolidate Telemetry Pipeline Statuses

Date: 2024-01-02

## Status

Accepted

## Context

When we began the project, we lacked familiarity with the API conventions outlined in [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) about Custom Resource Definition (CRD) statuses.
Instead, we made our own way by implementing a custom status condition structure, which was a subset of `metav1.Condition`.
Additionally, instead of treating status collections as a map with a key of type, we treated is as a dynamic list. At the start, a Pending condition was given to the resource upon creation. As underlying resources were deployed and ready, a Running condition was added. If issues arose, we cleared the entire list and only added a Pending condition. This deviates from API best practices and could be confusing for end users.

That approach was used for the `LogPipeline` and `TracePipeline` CRDs. Later on, we realized that we should follow the convention and use `metav1.Condition` instead. We did that for the `Telemetry` and `MetricPipeline` CRDs.

### Goal

Our goal is to standardize the status structure across all Custom Resource Definitions (CRDs) and adhere to API conventions. This will bring consistency in the status structure, making it more straightforward to manage and comprehend for end users.
What makes things tricky is that there may be Kyma customers who rely on the old status structure for `LogPipeline` and `TracePipeline` CRDs. We need to ensure that the transition is smooth and that we don't break anything.

## Decision

Roll out the change for `LogPipeline` and `TracePipeline` in multiple phases:
1. Augment the existing custom condition structures with the missing required fields from `metav1.Condition`, like `Status` and `Message`. Make them optional. Extend the controllers' logic to populate them with reasonable values. We can roll that out without bumping the `v1alpha1` API group, because we will only add optional fields.
2. Replace custom condition structures with `metav1.Condition`. Technically, it's a breaking change, because some optional fields become required. However, we can maintain the API version because the status is automatically reconciled and should never be directly set by a user.
3. Set new conditions in the status (`GatewayHealthy`/`AgentHealthy`, `ConfigurationGenerated`) and append old conditions to the list (`Pending`, `Running`). This way, we preserve the semantic, because the user would still infer a pipeline healthiness from the last condition in the list. Migrate to new conditions in E2E tests and `Telemetry` state propagation. Deprecate the old conditions and announce it in the Release Notes.
  ```yaml
status:
  conditions:
  - lastTransitionTime: "2024-02-01T08:26:02Z"
    message: Trace gateway Deployment is ready
    observedGeneration: 1
    reason: DeploymentReady
    status: "True"
    type: GatewayHealthy
  - lastTransitionTime: "2024-01-24T14:36:58Z"
    message: ""
    observedGeneration: 1
    reason: ConfigurationGenerated
    status: "True"
    type: ConfigurationGenerated
  - lastTransitionTime: "2024-02-01T08:24:02Z"
    message: Trace gateway Deployment is not ready
    observedGeneration: 1
    reason: DeploymentReady
    status: "False"
    type: Pending
  - lastTransitionTime: "2024-02-01T08:26:02Z"
    message: Trace gateway Deployment is ready
    observedGeneration: 1
    reason: DeploymentReady
    status: "True"
    type: Running
  ```
4. After the deprecation period, remove the old conditions from the list.

## Consequences

The status structure will be consistent across all CRDs, which will simplify our monitoring, as well as propagating the status from telemetry pipelines to the `Telemetry` CR. 
It will also make it easier to extend statuses with new condition types.

