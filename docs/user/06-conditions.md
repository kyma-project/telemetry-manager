# Telemetry CR conditions

This section describes the possible states of the Telemetry CR. There are conditions type `Logging`, `Metrics` and `Tracing` representing state of each of the subcomponents. The state of the telemetry CR is derived from combined state of all the subcomponents.

## Logging Subcomponent state
| No | Condition type | Condition status | Condition reason              | Remark                                     |
|----|----------------|------------------|-------------------------------|--------------------------------------------|
| 1  | Logging        | True             | ReasonNoPipelineDeployed      | No pipelines have been deployed            |
| 1  | Logging        | True             | ReasonFluentBitDSReady        | Fluent bit Daemonset is ready              |
| 1  | Logging        | False            | ReasonReferencedSecretMissing | One or more referenced secrets are missing |
| 1  | Logging        | False            | ReasonFluentBitDSNotReady     | Fluent bit Daemonset is not ready          |

## Tracing Subcomponent state
| No | Condition type | Condition status | Condition reason                       | Remark                                     |
|----|----------------|------------------|----------------------------------------|--------------------------------------------|
| 1  | Tracing        | True             | ReasonNoPipelineDeployed               | No pipelines have been deployed            |
| 1  | Tracing        | True             | ReasonTraceCollectorDeploymentReady    | Trace collector deployment is ready        |
| 1  | Tracing        | False            | ReasonReferencedSecretMissing          | One or more referenced secrets are missing |
| 1  | Tracing        | False            | ReasonTraceCollectorDeploymentNotReady | Trace collector is deployment not ready    |

## Metric Subcomponent state
| No | Condition type | Condition status | Condition reason                         | Remark                                     |
|----|----------------|------------------|------------------------------------------|--------------------------------------------|
| 1  | Metrics        | True             | ReasonNoPipelineDeployed                 | No pipelines have been deployed            |
| 1  | Metrics        | True             | ReasonMetricGatewayDeploymentReady       | Metric gateway deployment is ready         |
| 1  | Metrics        | False            | ReasonReferencedSecretMissing            | One or more referenced secrets are missing |
| 1  | Metrics        | False            | ReasonMetricGatewayDeploymentNotReady    | Metric gateway deployment is not ready     |


## Telemetry component state
| No | CR State | Logging State | Tracing State | Metrics State | 
|----|----------|---------------|---------------|---------------|
| 1  | Ready    | True          | True          | True          | 
| 1  | Warning  | False         | True          | True          |
| 1  | Warning  | False         | False         | False         |
| 1  | Warning  | False         | False         | True          |
| 1  | Warning  | False         | True          | False         |
