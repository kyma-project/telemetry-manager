# Telemetry CR conditions

This section describes the possible states of the Telemetry CR. There are conditions type `LogComponentsHealthy`, `TraceComponentsHealthy` and `MetricComponentsHealthy` representing state of each of the subcomponents. The state of the Telemetry CR is derived from combined state of all the subcomponents.

## Log Components State
| No | Condition type | Condition status | Condition reason              | Remark                                     |
|----|----------------|------------------|-------------------------------|--------------------------------------------|
| 1  | LogComponentsHealthy | True             | ReasonNoPipelineDeployed      | No pipelines have been deployed            |
| 2  | LogComponentsHealthy | True             | ReasonFluentBitDSReady        | Fluent Bit DaemonSet is ready              |
| 3  | LogComponentsHealthy | False            | ReasonReferencedSecretMissing | One or more referenced Secrets are missing |
| 4  | LogComponentsHealthy | False            | ReasonFluentBitDSNotReady     | Fluent Bit DaemonSet is not ready          |

## Trace Components State
| No | Condition type         | Condition status | Condition reason                       | Remark                                     |
|----|------------------------|------------------|----------------------------------------|--------------------------------------------|
| 1  | TraceComponentsHealthy | True             | ReasonNoPipelineDeployed               | No pipelines have been deployed            |
| 2  | TraceComponentsHealthy | True             | ReasonTraceCollectorDeploymentReady    | Trace collector Deployment is ready        |
| 3  | TraceComponentsHealthy | False            | ReasonReferencedSecretMissing          | One or more referenced Secrets are missing |
| 4  | TraceComponentsHealthy | False            | ReasonTraceCollectorDeploymentNotReady | Trace collector Deployment is not ready    |

## Metric Components State
| No | Condition type | Condition status | Condition reason                         | Remark                                     |
|----|----------------|------------------|------------------------------------------|--------------------------------------------|
| 1  | MetricComponentsHealthy | True             | ReasonNoPipelineDeployed                 | No pipelines have been deployed            |
| 2  | MetricComponentsHealthy | True             | ReasonMetricGatewayDeploymentReady       | Metric gateway Deployment is ready         |
| 3  | MetricComponentsHealthy | False            | ReasonReferencedSecretMissing            | One or more referenced Secrets are missing |
| 4  | MetricComponentsHealthy | False            | ReasonMetricGatewayDeploymentNotReady    | Metric gateway Deployment is not ready     |


## Telemetry CR State
| No | Telemetry State | LogComponentsHealthy | TraceComponentsHealthy | MetricComponentsHealthy | 
|----|-----------------|---------------|---------------|---------------|
| 1  | Ready           | True          | True          | True          | 
| 2  | Warning         | False         | True          | True          |
| 3  | Warning         | False         | False         | False         |
| 4  | Warning         | False         | False         | True          |
| 5  | Warning         | False         | True          | False         |
