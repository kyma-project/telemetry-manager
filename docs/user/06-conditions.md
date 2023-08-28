# Telemetry CR conditions

This section describes the possible states of the Telemetry CR. 
The state of the Telemetry CR is derived from the combined state of all the subcomponents, namely, from the condition types `LogComponentsHealthy`, `TraceComponentsHealthy` and `MetricComponentsHealthy`. 

## Log Components State

The state of the log components is determined by the status condition of type `LogComponentsHealthy`:

| Condition status | Condition reason              | Message                                    |
|------------------|-------------------------------|--------------------------------------------|
| True             | ReasonNoPipelineDeployed      | No pipelines have been deployed            |
| True             | ReasonFluentBitDSReady        | Fluent Bit DaemonSet is ready              |
| False            | ReasonReferencedSecretMissing | One or more referenced Secrets are missing |
| False            | ReasonFluentBitDSNotReady     | Fluent Bit DaemonSet is not ready          |

## Trace Components State

The state of the trace components is determined by the status condition of type `TraceComponentsHealthy`:

| Condition status | Condition reason                       | Message                                     |
|------------------|----------------------------------------|--------------------------------------------|
| True             | ReasonNoPipelineDeployed               | No pipelines have been deployed            |
| True             | ReasonTraceCollectorDeploymentReady    | Trace collector Deployment is ready        |
| False            | ReasonReferencedSecretMissing          | One or more referenced Secrets are missing |
| False            | ReasonTraceCollectorDeploymentNotReady | Trace collector Deployment is not ready    |

## Metric Components State

The state of the metric components is determined by the status condition of type `MetricComponentsHealthy`:

| Condition status | Condition reason                         | Message                                     |
|------------------|------------------------------------------|--------------------------------------------|
| True             | ReasonNoPipelineDeployed                 | No pipelines have been deployed            |
| True             | ReasonMetricGatewayDeploymentReady       | Metric gateway Deployment is ready         |
| False            | ReasonReferencedSecretMissing            | One or more referenced Secrets are missing |
| False            | ReasonMetricGatewayDeploymentNotReady    | Metric gateway Deployment is not ready     |


## Telemetry CR State

- 'Ready': Only if all the subcomponent conditions (LogComponentsHealthy, TraceComponentsHealthy, and MetricComponentsHealthy) have a status of 'True.' 
- 'Warning': If any of these conditions are not 'True'.
- 'Deleting': When a Telemetry CR is being deleted.
- 'Error': If the deletion is blocked because some dependent resources exist.
When a Telemetry CR is being deleted, its state is set to 'Deleting'. If the deletion is blocked due to the existence of some dependent resources, the state is changed to 'Error'.

