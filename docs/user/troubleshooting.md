# Troubleshooting for the Telemetry Module

Troubleshoot problems related to the Telemetry module and its pipelines.

## No Data Arrive at the Backend

**Symptom**:

- No data arrive at the backend.
- In the respective pipeline status, the `TelemetryFlowHealthy` condition has status **GatewayAllTelemetryDataDropped** or **AgentAllTelemetryDataDropped**.

**Cause**: Incorrect backend endpoint configuration (such as using the wrong authentication credentials) or the backend is unreachable.

**Solution**:

1. Check the error logs for the affected Pod by calling `kubectl logs -n kyma-system {POD_NAME}`:
   - For **GatewayAllTelemetryDataDropped**, check Pod `telemetry-(log|trace|metric)-gateway`.
   - For **AgentAllTelemetryDataDropped**, check Pod `telemetry-(log|trace|metric)-agent`.
2. Check if the backend is up and reachable.
3. Fix the errors.

## Not All Data Arrive at the Backend

**Symptom**:

- The backend is reachable and the connection is properly configured, but some data points are refused.
- In the pipeline status, the `TelemetryFlowHealthy` condition has status **GatewaySomeTelemetryDataDropped** or **AgentSomeTelemetryDataDropped**.

**Cause**: It can happen due to a variety of reasons - for example, the backend is limiting the ingestion rate.

**Solution**:

1. Check the error logs for the affected Pod by calling `kubectl logs -n kyma-system {POD_NAME}`:
   - For **GatewaySomeTelemetryDataDropped**, check Pod `telemetry-(log|trace|metric)-gateway`.
   - For **AgentSomeTelemetryDataDropped**, check Pod `telemetry-(log|trace|metric)-agent`.
2. Check your observability backend to investigate potential causes.
3. If the backend is limiting the rate by refusing logs, try these options:
   - Increase the ingestion rate of your backend (for example, by scaling out your SAP Cloud Logging instances).
   - Reduce emitted data by re-configuring the pipeline (for example, by disabling certain inputs or applying filters).
   - Reduce emitted data in your applications.
4. Otherwise, take the actions appropriate to the cause indicated in the logs.

## Gateway Throttling

**Symptom**: In the pipeline status, the `TelemetryFlowHealthy` condition has status **GatewayThrottling**.

**Cause**: Gateway cannot receive data at the given rate.

**Solution**: Manually scale out the gateway by increasing the number of replicas for the gateway. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).

### Custom Spans Don’t Arrive at the Backend, but Istio Spans Do

**Cause**: Your SDK version is incompatible with the OTel Collector version.

**Solution**:

1. Check which SDK version you are using for instrumentation.
2. Investigate whether it is compatible with the OTel Collector version.
3. If required, upgrade to a supported SDK version.

## Log Buffer Filling Up

**Symptom**: In the pipeline status, the `TelemetryFlowHealthy` condition has status **AgentBufferFillingUp**.

**Cause**: The backend ingestion rate is too low compared to the export rate of the gateway or agent.

**Solution**:

- Option 1: Increase maximum backend ingestion rate. For example, by scaling out the SAP Cloud Logging instances.

- Option 2: Reduce emitted data by re-configuring the pipeline (for example, by disabling certain inputs or applying namespace filters).

- Option 3: Reduce emitted data in your applications.

### Trace Backend Shows Fewer Traces than Expected

**Cause**: By default, only 1% of the requests are sent to the trace backend for trace recording, see [Traces Istio Support](./collecting-traces/istio-support.md).

**Solution**:

To see more traces in the trace backend, increase the percentage of requests by changing the default settings.
If you just want to see traces for one particular request, you can manually force sampling:

1. Create a `values.yaml` file.
   The following example sets the value to `60`, which means 60% of the requests are sent to the tracing backend.

```yaml
  apiVersion: telemetry.istio.io/v1
  kind: Telemetry
  metadata:
    name: kyma-traces
    namespace: istio-system
  spec:
    tracing:
    - providers:
      - name: "kyma-traces"
      randomSamplingPercentage: 60
```

2. To override the default percentage, change the value for the **randomSamplingPercentage** attribute.
3. Deploy the `values.yaml` to your existing Kyma installation.

### MetricPipeline: Failed to Scrape Prometheus Endpoint

**Symptom**: Custom metrics don't arrive at the destination. The OTel Collector produces log entries saying "Failed to scrape Prometheus endpoint", such as the following example:

```bash
2023-08-29T09:53:07.123Z warn internal/transaction.go:111 Failed to scrape Prometheus endpoint {"kind": "receiver", "name": "prometheus/app-pods", "data_type": "metrics", "scrape_timestamp": 1693302787120, "target_labels": "{__name__=\"up\", instance=\"10.42.0.18:8080\", job=\"app-pods\"}"}
```
<!-- markdown-link-check-disable-next-line -->
**Cause 1**: The workload is not configured to use 'STRICT' mTLS mode. For details, see [Metrics Prometheus Input](./prometheus-input.md).

**Solution 1**: You can either set up 'STRICT' mTLS mode or HTTP scraping:

- Configure the workload using “STRICT” mTLS mode (for example, by applying a corresponding PeerAuthentication).
- Set up scraping through HTTP by applying the `prometheus.io/scheme=http` annotation.
<!-- markdown-link-check-disable-next-line -->
**Cause 2**: The Service definition enabling the scrape with Prometheus annotations does not reveal the application protocol to use in the port definition. For details, see [Metrics Prometheus Input](./prometheus-input.md).

**Solution 2**: Define the application protocol in the Service port definition by either prefixing the port name with the protocol, like in `http-metrics` or define the `appProtocol` attribute.

**Cause 3**: A deny-all `NetworkPolicy` was created in the workload namespace, which prevents that the agent can scrape metrics from annotated workloads.

**Solution 3**: Create a separate `NetworkPolicy` to explicitly let the agent scrape your workload using the `telemetry.kyma-project.io/metric-scrape` label.

For example, see the following `NetworkPolicy` configuration:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-traffic-from-agent
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: "annotated-workload" # <your workload here>
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kyma-system
      podSelector:
        matchLabels:
          telemetry.kyma-project.io/metric-scrape: "true"
  policyTypes:
  - Ingress
```
