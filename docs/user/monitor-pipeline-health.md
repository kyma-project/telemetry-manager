# Monitor Pipeline Health

The Telemetry module is designed to be reliable and resilient. However, there may be situations when the instances drop data or cannot handle the load, and you must take action.

## Overview

The Telemetry module automatically handles temporary issues to prevent data loss and ensure that the OTel Collector instances of your pipelines are operational and healthy. For example, if your backend is temporarily unavailable, the module buffers your data and attempts to resend it when the connection is restored.

Telemetry module continuously monitors the health of your pipelines (see [Self Monitor](../architecture/README.md#self-monitor)). To ensure that your Telemetry pipelines operate reliably, you can monitor their health data in the following ways:

- Perform manual checks by inspecting the status conditions of your pipeline resources with `kubectl`.
- Set up continuous monitoring by using a `MetricPipeline` to export health metrics to your observability backend, where you can set up dashboards and alerts.

## Check Pipeline Status

For a quick check, you can inspect the `status` of a pipeline resource directly.

1. Run `kubectl get` for the pipeline that you want to inspect:
   - For `LogPipeline`: `kubectl get logpipeline <your-pipeline-name>`
   - For `TracePipeline`: `kubectl get tracepipeline <your-pipeline-name>`
   - For `MetricPipeline`: `kubectl get tracepipeline <your-pipeline-name>`
2. Review the output. A healthy pipeline shows `True` for all status conditions.

    ```txt
    NAME      CONFIGURATION GENERATED   GATEWAY HEALTHY   FLOW HEALTHY
    backend   True                      True              True
    ```

3. If any condition is `False`, investigate problem and fix it.

To understand the meaning of each status condition, see the detailed reference for each pipeline type:

- [LogPipeline Status](https://kyma-project.io/#/telemetry-manager/user/resources/02-logpipeline?id=logpipeline-status)
- [TracePipeline Status](https://kyma-project.io/#/telemetry-manager/user/resources/04-tracepipeline?id=tracepipeline-status)
- [MetricPipeline Status](https://kyma-project.io/#/telemetry-manager/user/resources/05-metricpipeline?id=metricpipeline-status)

## Set Up Health Monitoring and Alerts

For production environments, set up continuous monitoring by exporting the health metrics to your observability backend, where you can create dashboards and configure alerts using alert rules. For an example, see [Integrate With SAP Cloud Logging](../integration/sap-cloud-logging/README.md)

> **Caution:** Don't access the metrics endpoint of the used OTel Collector instances directly, because the exposed metrics are no official API of the Telemetry module. Breaking changes can happen if the underlying OTel Collector version introduces such. Instead, use the respective status conditions for each pipeline.

To collect these health metrics, you must have at least one active `MetricPipeline` in your cluster. This pipeline automatically collects and exports health data for all of your pipelines, including `LogPipeline` and `TracePipeline` resources.

The Telemetry module emits the following metrics for health monitoring:

- `kyma.resource.status.conditions`: Represents the status of a specific condition on a resource. It is available for all pipelines and the main `Telemetry` resource.
  Values: `1` ("True"), `0` ("False"), or `-1` ("Unknown")
  Specific attributes:
  - `metric.attributes.type`: The type of the status condition
  - `metric.attributes.status`: The status of the condition
  - `metric.attributes.reason`: A programmatic identifier indicating the reason for the condition's last transition
- `kyma.resource.status.state`: Represents the overall state of the main `Telemetry` resource.
  Values: `1` ("Ready") or `0` ("Not Ready")
  Specific attributes: `state`: The value of the `status.state` field
- Additionally, the following attributes are attached to all health metrics to identify the source resource:
  - `k8s.resource.group`: The group of the resource
  - `k8s.resource.version`: The version of the resource
  - `k8s.resource.kind`: The kind of the resource
  - `k8s.resource.name`: The name of the resource

To create an alert, define a rule that triggers on a specific metric value. For example, to create an alert that fires if a pipeline's `TelemetryFlowHealthy` condition becomes "False" (indicating data flow issues), use the following PromQL query:

```txt
min by (k8s_resource_name) ((kyma_resource_status_conditions{type="TelemetryFlowHealthy",k8s_resource_kind="metricpipelines"})) == 0
```

If there are issues with one of the pipelines, see [Troubleshooting for the Telemetry Module](ADD LINK).
