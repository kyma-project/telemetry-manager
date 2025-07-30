# Troubleshooting

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

### Not All Data Arrive at the Backend

**Symptom**:

- The backend is reachable and the connection is properly configured, but some data points are refused.
- In the pipeline status, the `TelemetryFlowHealthy` condition has status **GatewaySomeTelemetryDataDropped** or **AgentSomeTelemetryDataDropped**.

**Cause**: It can happen due to a variety of reasons - for example, the backend is limiting the ingestion rate.

**Solution**:

1. Check the error logs for the affected Pod by calling `kubectl logs -n kyma-system {POD_NAME}`:
   - For **GatewaySomeTelemetryDataDropped**, check Pod `telemetry-(log|trace|metric)-gateway`.
   - For **AgentSomeTelemetryDataDropped**, check Pod `telemetry-(log|trace|metric)-agent`.
2. Check your observability backend to investigate potential causes.
3. If the backend is limiting the rate by refusing logs, try the options described in [Buffer Filling Up](#buffer-filling-up).
4. Otherwise, take the actions appropriate to the cause indicated in the logs.

### Buffer Filling Up

**Symptom**: In the pipeline status, the `TelemetryFlowHealthy` condition has status **GatewayBufferFillingUp** or **AgentBufferFillingUp**.

**Cause**: The backend ingestion rate is too low compared to the export rate of the gateway or agent.

**Solution**:

- Option 1: Increase maximum backend ingestion rate. For example, by scaling out the SAP Cloud Logging instances.

- Option 2: Reduce emitted data by re-configuring the pipeline (for example, by disabling certain inputs or applying namespace filters).

- Option 3: Reduce emitted data in your applications.

### Gateway Throttling

**Symptom**: In the pipeline status, the `TelemetryFlowHealthy` condition has status **GatewayThrottling**.

**Cause**: Gateway cannot receive data at the given rate.

**Solution**: Manually scale out the gateway by increasing the number of replicas for the gateway. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).
