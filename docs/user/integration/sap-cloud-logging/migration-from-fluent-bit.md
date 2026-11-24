# Migrate from Fluent Bit to the OTel Log Pipeline (SAP Cloud Logging)

If you have an existing Fluent Bit-based integration with SAP Cloud Logging (CLS), use this guide to migrate to the OTel-based log pipeline. The OTel pipeline consolidates your two Fluent Bit LogPipelines into one, simplifies secret management, and routes logs to the unified `logs-otel-v1-*` index used by the predefined **OpenTelemetry** dashboards in CLS.

You can run the old and new pipelines in parallel during the migration — they write to separate indices and don't produce duplicates, so there's no risk of data loss or gaps in your logs.

## What Changes

| | Before (Fluent Bit) | After (OTel) |
|---|---|---|
| LogPipelines for logs | 2 (minimum) | 1 |
| Secret keys | `ingest-mtls-endpoint`, `ingest-mtls-cert`, `ingest-mtls-key` | `ingest-otlp-endpoint`, `ingest-otlp-cert`, `ingest-otlp-key` |
| OpenSearch index (app logs) | `logs-json-kyma-*` | `logs-otel-v1-*` |
| OpenSearch index (Istio access logs) | `logs-json-istio-envoy-kyma-*` | `logs-otel-v1-*` |
| Istio extension provider | `stdout-json` | `kyma-logs` |
| Predefined CLS dashboards | **Kyma** section | **OpenTelemetry** section |
| Kyma dashboard ConfigMap | `kyma-dashboard-http-configmap.yaml` | `kyma-dashboard-configmap.yaml` |

## Prerequisites

- You have an existing Fluent Bit-based CLS integration with the `sap-cloud-logging-application-logs` and `sap-cloud-logging-access-logs` LogPipelines deployed.
- You have permission to update your CLS service instance parameters and create new ServiceBindings.

> [!WARNING]
> After enabling OTLP on your CLS instance, you must create a new ServiceBinding (or service key). The `ingest-otlp-*` secret keys are only present in bindings created **after** OTLP ingestion was enabled on the instance.

## Procedure

1. Enable OTLP ingestion on your existing CLS instance by updating the service instance parameter:

   ```json
   {
     "ingest_otlp": {
       "enabled": true
     }
   }
   ```

   No new CLS instance is needed. For details, see [Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters).

2. Create a new ServiceBinding to get the `ingest-otlp-*` secret keys. Make the resulting secret available in your cluster under the same name and namespace as the existing `sap-cloud-logging` secret.

3. Create the new OTel LogPipeline alongside your existing Fluent Bit pipelines:

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: telemetry.kyma-project.io/v1beta1
   kind: LogPipeline
   metadata:
     name: sap-cloud-logging
   spec:
     input:
       runtime:
         enabled: true
       otlp:
         enabled: true
     output:
       otlp:
         endpoint:
           valueFrom:
             secretKeyRef:
               name: sap-cloud-logging
               namespace: sap-cloud-logging-integration
               key: ingest-otlp-endpoint
         tls:
           cert:
             valueFrom:
               secretKeyRef:
                 name: sap-cloud-logging
                 namespace: sap-cloud-logging-integration
                 key: ingest-otlp-cert
           key:
             valueFrom:
               secretKeyRef:
                 name: sap-cloud-logging
                 namespace: sap-cloud-logging-integration
                 key: ingest-otlp-key
   EOF
   ```

4. Reconfigure Istio to use the `kyma-logs` extension provider. Update your existing Istio Telemetry resource — replace `stdout-json` with `kyma-logs`:

   ```yaml
   spec:
     accessLogging:
       - providers:
         - name: kyma-logs
   ```

   The `kyma-logs` provider emits access logs as OTel log records with semantic-convention attributes, which are compatible with the `logs-otel-v1-*` index. The `stdout-json` provider emits Envoy-native JSON that an OTel LogPipeline can't receive. For details on Istio access log configuration, see [Collect Istio Access Logs](./../../collecting-logs/istio-support.md).

5. Verify the new pipeline is healthy:

   ```bash
   kubectl get logpipeline sap-cloud-logging
   ```

   Wait until the pipeline shows `Running` status. Then open the **OpenTelemetry** section in OpenSearch Dashboards and confirm application logs and Istio access logs are arriving in the `logs-otel-v1-*` index.

   > [!NOTE]
   > Don't proceed to the next step until verification succeeds. Your old Fluent Bit pipelines are still running — if the new pipeline isn't healthy, troubleshoot it before removing the old ones.

6. Delete the old Fluent Bit LogPipelines:

   ```bash
   kubectl delete logpipeline sap-cloud-logging-application-logs
   kubectl delete logpipeline sap-cloud-logging-access-logs
   ```

   If you have additional Fluent Bit pipelines targeting CLS, delete those too.

7. Update the Kyma dashboard ConfigMap:

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/kyma-dashboard-configmap.yaml
   ```

8. Switch to the predefined OpenTelemetry dashboards in CLS. The **Kyma** dashboard section is no longer populated after the migration. Use the **OpenTelemetry** section in OpenSearch Dashboards for application and Istio access logs going forward.

9. Replace predefined dashboards and alert monitors. The predefined dashboards and monitors from the Fluent Bit integration are based on the `logs-json-kyma-*` and `logs-json-istio-envoy-kyma-*` indices and no longer receive data after the migration. Delete them in OpenSearch Dashboards and re-import the current versions from the [integration guide](./README.md#use-sap-cloud-logging-alerts).

10. Migrate custom OpenSearch saved objects and monitors. If you created custom dashboards, visualizations, or alert monitors based on the old Fluent Bit indices or log record structure, update them as follows:

    - **Saved objects** (dashboards, visualizations, index patterns): Update index patterns from `logs-json-kyma-*` and `logs-json-istio-envoy-kyma-*` to `logs-otel-v1-*`. Update any field references — the log record structure changed from Envoy-native JSON to OTel semantic-convention attributes. For the full attribute reference, see the [OTel Log Data Model](https://opentelemetry.io/docs/specs/otel/logs/data-model/).
    - **Alert monitors**: Update the index in the monitor query from `logs-json-kyma-*` or `logs-json-istio-envoy-kyma-*` to `logs-otel-v1-*`. Update any field-level conditions to match the new OTel attribute names.

## Result

Your cluster sends all logs through a single OTel LogPipeline to CLS. Logs appear in the `logs-otel-v1-*` index, and the predefined **OpenTelemetry** dashboards in CLS are populated. The legacy Fluent Bit pipelines, their `ingest-mtls-*` credentials, and the old Fluent Bit-based dashboards and monitors are no longer in use.

## Related Information

- [Set Up SAP Cloud Logging](./README.md)
- [Collect Istio Access Logs](./../../collecting-logs/istio-support.md)
- [OTel Log Data Model](https://opentelemetry.io/docs/specs/otel/logs/data-model/)
- [CLS Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters)
