# Telemetry Health Input

By default, a MetricPipeline emits metrics about the health of all pipeline resources taken from each status. Based on these status metrics, you can track the status of every individual pipeline and set up alerting for it.

For every condition for every pipeline there will be a `kyma.resource.status.conditions` metric.
Having a pipeline with the following status (in real there are more conditions):

```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: LogPipeline
metadata:
  name: test
status:
  conditions:
  - lastTransitionTime: "2025-07-28T20:59:19Z"
    message: No problems detected in the telemetry flow
    observedGeneration: 2
    reason: FlowHealthy
    status: "True"
    type: TelemetryFlowHealthy
```

you will get a metric like this:

```yaml
name: kyma.resource.status.conditions
  resource.attributes:
    k8s.resource.group: telemetry.kyma-project.io
    k8s.resource.version: v1alpha1
    k8s.resource.kind: logpipelines
    k8s.resource.name: test
  attributes:
    type: TelemetryFlowHealthy
    status: "True"
    reason: FlowHealthy
  value: 1
```

Additionally, there is the `kyma.resource.status.state` metric which will be instrumented only for the Telemetry resource. For a telemetry resource like this:

```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Telemetry
metadata:
  name: default
status:
  conditions:
  - lastTransitionTime: "2025-07-21T14:54:18Z"
    message: All trace components are running
    observedGeneration: 17
    reason: ComponentsRunning
    status: "True"
    type: TraceComponentsHealthy
  state: Ready
```

The following metrics will be instrumented:

```yaml
name: kyma.resource.status.state
  resource.attributes:
    k8s.resource.group: operator.kyma-project.io
    k8s.resource.version: v1alpha1
    k8s.resource.kind: telemetries
    k8s.resource.name: default
  attributes:
    state: Ready
  value: 1
name: kyma.resource.status.conditions
  resource.attributes:
    k8s.resource.group: operator.kyma-project.io
    k8s.resource.version: v1alpha1
    k8s.resource.kind: telemetries
    k8s.resource.name: default
  attributes:
    type: TraceComponentsHealthy
    status: "True"
    reason: ComponentsRunning
  value: 1
```

## Metrics

Metrics with attributes instrumented by the input:

| Metric Name | Metric Attribute | Description | Availability |
|--|--|--|--|
| kyma.resource.status.conditions | | Value represents status of different conditions reported by the resource.  Possible values are 1 ("True"), 0 ("False"), and -1 (other status values) | Available for both, the pipelines and the Telemetry resource |
| | type   | Type of the status condition | |
| | status | Status of the status condition | |
| | reason | Contains a programmatic identifier indicating the reason for the condition's last transition | |
| kyma.resource.status.state | | Value represents the state of the resource (if present). Possible values are 1 ("Ready") or 0 | Available for the Telemetry resource only |
| | state   | value of the status.state | |

Additionally, all metrics will have attached these resource attributes, identifying the related resource:

| Attribute | Description |
|--|--|
| k8s.resource.group | Group of the resource |
| k8s.resource.version | Version of the resource kind |
| k8s.resource.kind | Kind of the resource |
| k8s.resource.name | Name of the resource |

## Alerting

Usually you want to alert on a negative health of a pipeline. In the following example, a promql based alert is triggered if metrics are not delivered to the backend:

```promql
 min by (k8s_resource_name) ((kyma_resource_status_conditions{type="TelemetryFlowHealthy",k8s_resource_kind="metricpipelines"})) == 0
```
