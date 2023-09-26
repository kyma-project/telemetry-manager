# Telemetry CR conditions

This section describes the possible states of the Telemetry CR. 
The state of the Telemetry CR is derived from the combined state of all the subcomponents, namely, from the condition types `LogComponentsHealthy`, `TraceComponentsHealthy` and `MetricComponentsHealthy`. 

## Log Components State

The state of the log components is determined by the status condition of type `LogComponentsHealthy`:

| Condition status | Condition reason        | Message                                         |
|------------------|-------------------------|-------------------------------------------------|
| True             | NoPipelineDeployed      | No pipelines have been deployed                 |
| True             | FluentBitDaemonSetReady | Fluent Bit DaemonSet is ready                   |
| False            | ReferencedSecretMissing | One or more referenced Secrets are missing      |
| False            | FluentBitDaemonSetNotReady | Fluent Bit DaemonSet is not ready               |
| False            | ResourceBlocksDeletion  | The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (resource-1, resource-2,...), LogParsers (resource-1, resource-2,...) |


## Trace Components State

The state of the trace components is determined by the status condition of type `TraceComponentsHealthy`:

| Condition status | Condition reason          | Message                                    |
|------------------|---------------------------|--------------------------------------------|
| True             | NoPipelineDeployed        | No pipelines have been deployed            |
| True             | TraceGatewayDeploymentReady | Trace gateway Deployment is ready          |
| False            | ReferencedSecretMissing   | One or more referenced Secrets are missing |
| False            | TraceGatewayDeploymentNotReady | Trace gateway Deployment is not ready      |
| False            | ResourceBlocksDeletion    | The deletion of the module is blocked. To unblock the deletion, delete the following resources: TracePipelines (resource-1, resource-2,...) |

## Metric Components State

The state of the metric components is determined by the status condition of type `MetricComponentsHealthy`:

| Condition status | Condition reason          | Message                                    |
|------------------|---------------------------|--------------------------------------------|
| True             | NoPipelineDeployed        | No pipelines have been deployed            |
| True             | MetricGatewayDeploymentReady | Metric gateway Deployment is ready         |
| False            | ReferencedSecretMissing   | One or more referenced Secrets are missing |
| False            | MetricGatewayDeploymentNotReady | Metric gateway Deployment is not ready     |
| False            | ResourceBlocksDeletion    | The deletion of the module is blocked. To unblock the deletion, delete the following resources: MetricPipelines (resource-1, resource-2,...)    |


## Telemetry CR State

- 'Ready': Only if all the subcomponent conditions (LogComponentsHealthy, TraceComponentsHealthy, and MetricComponentsHealthy) have a status of 'True.' 
- 'Warning': If any of these conditions are not 'True'.
- 'Deleting': When a Telemetry CR is being deleted.
