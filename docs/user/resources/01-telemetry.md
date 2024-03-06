# Telemetry

The `telemetry.operator.kyma-project.io` CustomResourceDefinition (CRD) is a detailed description of the kind of data and the format used to define a Telemetry module instance. To get the current CRD and show the output in the YAML format, run this command:

```bash
kubectl get crd telemetry.operator.kyma-project.io -o yaml
```

## Sample Custom Resource

The following Telemetry object defines a module`:

```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
  generation: 2
Status:
  state: Ready
  endpoints:
    traces:
      grpc: http://telemetry-otlp-traces.kyma-system:4317
      http: http://telemetry-otlp-traces.kyma-system:4318
    metrics:
      grpc: http://telemetry-otlp-metrics.kyma-system:4317
      http: http://telemetry-otlp-metrics.kyma-system:4318
  conditions:
  - lastTransitionTime: "2023-09-01T15:28:28Z"
    message: All log components are running
    observedGeneration: 2
    reason: LogComponentsRunning
    status: "True"
    type: LogComponentsHealthy
  - lastTransitionTime: "2023-09-01T15:46:59Z"
    message: All metric components are running
    observedGeneration: 2
    reason: MetricComponentsRunning
    status: "True"
    type: MetricComponentsHealthy
  - lastTransitionTime: "2023-09-01T15:35:38Z"
    message: All trace components are running
    observedGeneration: 2
    reason: TraceComponentsRunning
    status: "True"
    type: TraceComponentsHealthy

```

For further examples, see the [samples](https://github.com/kyma-project/telemetry-manager/tree/main/config/samples) directory.

## Custom resource parameters

For details, see the [Telemetry specification file](https://github.com/kyma-project/telemetry-manager/blob/main/apis/operator/v1alpha1/telemetry_types.go).

<!-- The table below was generated automatically -->
<!-- Some special tags (html comments) are at the end of lines due to markdown requirements. -->
<!-- The content between "TABLE-START" and "TABLE-END" will be replaced -->

<!-- TABLE-START -->
### Telemetry.operator.kyma-project.io/v1alpha1

**Spec:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **metric**  | object | MetricSpec defines the behavior of the metric gateway |
| **metric.&#x200b;gateway**  | object |  |
| **metric.&#x200b;gateway.&#x200b;scaling**  | object | Scaling defines which strategy is used for scaling the gateway, with detailed configuration options for each strategy type. |
| **metric.&#x200b;gateway.&#x200b;scaling.&#x200b;static**  | object | Static is a scaling strategy allowing you to define a custom amount of replicas to be used for the gateway. Present only if Type = StaticScalingStrategyType. |
| **metric.&#x200b;gateway.&#x200b;scaling.&#x200b;static.&#x200b;replicas**  | integer | Replicas defines a static number of pods to run the gateway. Minimum is 1. |
| **metric.&#x200b;gateway.&#x200b;scaling.&#x200b;type**  | string | Type of scaling strategy. Default is none, using a fixed amount of replicas. |
| **trace**  | object | TraceSpec defines the behavior of the trace gateway |
| **trace.&#x200b;gateway**  | object |  |
| **trace.&#x200b;gateway.&#x200b;scaling**  | object | Scaling defines which strategy is used for scaling the gateway, with detailed configuration options for each strategy type. |
| **trace.&#x200b;gateway.&#x200b;scaling.&#x200b;static**  | object | Static is a scaling strategy allowing you to define a custom amount of replicas to be used for the gateway. Present only if Type = StaticScalingStrategyType. |
| **trace.&#x200b;gateway.&#x200b;scaling.&#x200b;static.&#x200b;replicas**  | integer | Replicas defines a static number of pods to run the gateway. Minimum is 1. |
| **trace.&#x200b;gateway.&#x200b;scaling.&#x200b;type**  | string | Type of scaling strategy. Default is none, using a fixed amount of replicas. |

**Status:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **conditions**  | \[\]object | Conditions contain a set of conditionals to determine the State of Status. If all Conditions are met, State is expected to be in StateReady. |
| **conditions.&#x200b;lastTransitionTime** (required) | string | lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable. |
| **conditions.&#x200b;message** (required) | string | message is a human readable message indicating details about the transition. This may be an empty string. |
| **conditions.&#x200b;observedGeneration**  | integer | observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance. |
| **conditions.&#x200b;reason** (required) | string | reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty. |
| **conditions.&#x200b;status** (required) | string | status of the condition, one of True, False, Unknown. |
| **conditions.&#x200b;type** (required) | string | type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt) |
| **endpoints**  | object | endpoints for trace and metric gateway. |
| **endpoints.&#x200b;metrics**  | object | metrics contains the endpoints for metric gateway supporting OTLP. |
| **endpoints.&#x200b;metrics.&#x200b;grpc**  | string | GRPC endpoint for OTLP. |
| **endpoints.&#x200b;metrics.&#x200b;http**  | string | HTTP endpoint for OTLP. |
| **endpoints.&#x200b;traces**  | object | traces contains the endpoints for trace gateway supporting OTLP. |
| **endpoints.&#x200b;traces.&#x200b;grpc**  | string | GRPC endpoint for OTLP. |
| **endpoints.&#x200b;traces.&#x200b;http**  | string | HTTP endpoint for OTLP. |
| **state** (required) | string | State signifies current state of Module CR. Value can be one of these three: "Ready", "Deleting", or "Warning". |

<!-- TABLE-END -->

The `state` attribute of the Telemetry CR is derived from the combined state of all the subcomponents, namely, from the condition types `LogComponentsHealthy`, `TraceComponentsHealthy` and `MetricComponentsHealthy`. 

### Log Components State

The state of the log components is determined by the status condition of type `LogComponentsHealthy`:

| Condition Status | Condition Reason                   | Condition Message                                                                                                                                                                                                                                         |
|------------------|------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| True             | NoPipelineDeployed                 | No pipelines have been deployed                                                                                                                                                                                                                           |
| True             | LogComponentsRunning               | All log components are running                                                                                                                                                                                                                            |
| False            | LogPipelineReferencedSecretMissing | One or more referenced Secrets are missing                                                                                                                                                                                                                |
| False            | FluentBitDaemonSetNotReady         | Fluent Bit DaemonSet is not ready                                                                                                                                                                                                                         |
| False            | ResourceBlocksDeletion             | The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (resource-1, resource-2,...), LogParsers (resource-1, resource-2,...)                                                                        |
| False            | UnsupportedLokiOutput              | The grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow [Installing a custom Loki stack in Kyma](https://kyma-project.io/#/telemetry-manager/user/integration/loki/README). |

### Trace Components State

The state of the trace components is determined by the status condition of type `TraceComponentsHealthy`:

| Condition Status | Condition Reason                     | Condition Message                                                                                                                           |
|------------------|--------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| True             | NoPipelineDeployed                   | No pipelines have been deployed                                                                                                             |
| True             | TraceComponentsRunning               | All trace components are running                                                                                                            |
| False            | TracePipelineReferencedSecretMissing | One or more referenced Secrets are missing                                                                                                  |
| False            | TraceGatewayDeploymentNotReady       | Trace gateway Deployment is not ready                                                                                                       |
| False            | ResourceBlocksDeletion               | The deletion of the module is blocked. To unblock the deletion, delete the following resources: TracePipelines (resource-1, resource-2,...) |
| False            | MaxPipelinesExceeded                 | Maximum pipeline count exceeded                                                                                                             |

### Metric Components State

The state of the metric components is determined by the status condition of type `MetricComponentsHealthy`:

| Condition Status | Condition Reason                      | Condition Message                                                                                                                            |
|------------------|---------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| True             | NoPipelineDeployed                    | No pipelines have been deployed                                                                                                              |
| True             | MetricComponentsRunning               | All metric components are running                                                                                                            |
| False            | MetricPipelineReferencedSecretMissing | One or more referenced Secrets are missing                                                                                                   |
| False            | MetricGatewayDeploymentNotReady       | Metric gateway Deployment is not ready                                                                                                       |
| False            | MetricAgentDaemonSetNotReady          | Metric agent DaemonSet is not ready                                                                                                          |
| False            | ResourceBlocksDeletion                | The deletion of the module is blocked. To unblock the deletion, delete the following resources: MetricPipelines (resource-1, resource-2,...) |
| False            | MaxPipelinesExceeded                 | Maximum pipeline count exceeded                                                                                                             |

### Telemetry CR State

- 'Ready': Only if all the subcomponent conditions (LogComponentsHealthy, TraceComponentsHealthy, and MetricComponentsHealthy) have a status of 'True.' 
- 'Warning': If any of these conditions are not 'True'.
- 'Deleting': When a Telemetry CR is being deleted.
