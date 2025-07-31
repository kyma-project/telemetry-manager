# Telemetry Health Input

By default, a MetricPipeline emits metrics about the health of all pipelines managed by the Telemetry module. Based on these metrics, you can track the status of every individual pipeline and set up alerting for it.

## Metrics

Metrics for Pipelines and the Telemetry Module:

| Metric                          | Description                                                                                                                                              | Availability                                                |
|---------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------|
| kyma.resource.status.conditions | Value represents status of different conditions reported by the resource.  Possible values are 1 ("True"), 0 ("False"), and -1 (other status values) | Available for both, the pipelines and the Telemetry resource |
| kyma.resource.status.state      | Value represents the state of the resource (if present)                                                                                                  | Available for the Telemetry resource                        |

Metric Attributes for Monitoring:

| Name                     | Description                                                                                  |
|--------------------------|----------------------------------------------------------------------------------------------|
| metric.attributes.Type   | Type of the condition                                                                        |
| metric.attributes.status | Status of the condition                                                                      |
| metric.attributes.reason | Contains a programmatic identifier indicating the reason for the condition's last transition |

## Alerting

Usually you want to alert on a negative health of a pipeline. In the following example, a promql based alert is triggered if metrics are not delivered to the backend:

```promql
 min by (k8s_resource_name) ((kyma_resource_status_conditions{type="TelemetryFlowHealthy",k8s_resource_kind="metricpipelines"})) == 0
```
