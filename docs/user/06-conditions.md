# Telemetry CR conditions

This section describes the possible states of the Telemetry CR. 
The state of the Telemetry CR is derived from the combined state of all the subcomponents, namely, from the condition types `LogComponentsHealthy`, `TraceComponentsHealthy` and `MetricComponentsHealthy`. 

## Log Components State

The state of the log components is determined by the status condition of type `LogComponentsHealthy`:

| Condition status | Condition reason                | Message                                         |
|------------------|---------------------------------|-------------------------------------------------|
| True             | ReasonNoPipelineDeployed        | No pipelines have been deployed                 |
| True             | ReasonFluentBitDSReady          | Fluent Bit DaemonSet is ready                   |
| False            | ReasonReferencedSecretMissing   | One or more referenced Secrets are missing      |
| False            | ReasonFluentBitDSNotReady       | Fluent Bit DaemonSet is not ready               |
| False            | ReasonLogResourceBlocksDeletion | One or more LogPipelines/LogParsers still exist |


## Trace Components State

The state of the trace components is determined by the status condition of type `TraceComponentsHealthy`:

| Condition status | Condition reason                     | Message                                    |
|------------------|--------------------------------------|--------------------------------------------|
| True             | ReasonNoPipelineDeployed             | No pipelines have been deployed            |
| True             | ReasonTraceGatewayDeploymentReady    | Trace gateway Deployment is ready          |
| False            | ReasonReferencedSecretMissing        | One or more referenced Secrets are missing |
| False            | ReasonTraceGatewayDeploymentNotReady | Trace gateway Deployment is not ready      |
| False            | ReasonTraceResourceBlocksDeletion    | One or more TracePipelines still exist     |

## Metric Components State

The state of the metric components is determined by the status condition of type `MetricComponentsHealthy`:

| Condition status | Condition reason                        | Message                                     |
|------------------|-----------------------------------------|--------------------------------------------|
| True             | ReasonNoPipelineDeployed                | No pipelines have been deployed            |
| True             | ReasonMetricGatewayDeploymentReady      | Metric gateway Deployment is ready         |
| False            | ReasonReferencedSecretMissing           | One or more referenced Secrets are missing |
| False            | ReasonMetricGatewayDeploymentNotReady   | Metric gateway Deployment is not ready     |
| False            | ReasonMetricResourceBlocksDeletion      | One or more MetricPipelines still exist     |


## Telemetry CR State

- 'Ready': Only if all the subcomponent conditions (LogComponentsHealthy, TraceComponentsHealthy, and MetricComponentsHealthy) have a status of 'True.' 
- 'Warning': If any of these conditions are not 'True'.
- 'Deleting': When a Telemetry CR is being deleted.
