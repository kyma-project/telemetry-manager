# Migrate from Fluent Bit to the OTel Log Pipeline (SAP Cloud Logging)

## Context

If you have an existing Fluent Bit-based integration with SAP Cloud Logging, you likely have at least two LogPipelines routing logs to two custom OpenSearch indices that are part of the predefined **Kyma** dashboard section in SAP Cloud Logging:

| Pipeline | URI | OpenSearch index |
|---|---|---|
| `sap-cloud-logging-application-logs` | `/customindex/kyma` | `logs-json-kyma-*` |
| `sap-cloud-logging-access-logs` | `/customindex/istio-envoy-kyma` | `logs-json-istio-envoy-kyma-*` |

Both pipelines used the `ingest-mtls-endpoint`, `ingest-mtls-cert`, and `ingest-mtls-key` Secret keys. Istio access logs were collected by Fluent Bit from stdout, so Istio had to be configured with the `stdout-json` extension provider.

The new OTel-based setup differs in several ways:

- A single LogPipeline replaces both, using the `ingest-otlp-endpoint`, `ingest-otlp-cert`, and `ingest-otlp-key` Secret keys
- All logs land in the `logs-otel-v1-*` index
- Istio must use the `kyma-logs` extension provider instead of `stdout-json`. The `kyma-logs` provider emits access logs as OTel log records with semantic-convention attributes, which are compatible with the `logs-otel-v1-*` index and the **OpenTelemetry** dashboards in SAP Cloud Logging. The `stdout-json` provider emits Envoy-native JSON that was designed for the legacy **Kyma** dashboards and cannot be received by an OTel LogPipeline at all.
- The predefined **Kyma** dashboards in SAP Cloud Logging are built for the old Fluent Bit index format and are not compatible with OTel logs — use the **OpenTelemetry** section in OpenSearch Dashboards instead

## Prerequisites

- Your SAP Cloud Logging instance has OTLP ingestion enabled. You can enable it on an existing instance by updating the service instance parameter `ingest_otlp.enabled: true` — no new instance is needed. Note that `ingest_otlp` is a separately billed component.
- After enabling OTLP on the instance, create a new ServiceBinding (or service key). The `ingest-otlp-*` Secret keys are only present in bindings created **after** OTLP was enabled.
- Running both the old Fluent Bit pipelines and the new OTel pipeline in parallel on the same CLS instance is safe — they write to different indices and do not produce duplicates.

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

## Procedure

1. **Enable OTLP ingestion** on your existing CLS instance by updating the service instance parameter:

   ```json
   {
     "ingest_otlp": {
       "enabled": true
     }
   }
   ```

   For details, see [Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters).

2. **Create a new ServiceBinding** to get the `ingest-otlp-*` Secret keys, and make the resulting Secret available in your cluster under the name and namespace you use for the existing `sap-cloud-logging` Secret.

3. **Create the new OTel LogPipeline** alongside your existing ones:

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

4. **Reconfigure Istio** to use the `kyma-logs` provider. Update your existing Istio `Telemetry` resource — replace `stdout-json` with `kyma-logs`:

   ```yaml
   spec:
     accessLogging:
       - providers:
         - name: kyma-logs
   ```

   For details, see [Configure Istio Access Logs](./../../collecting-logs/istio-support.md).

5. **Verify** the new pipeline is healthy and logs appear in SAP Cloud Logging:

   ```bash
   kubectl get logpipeline sap-cloud-logging
   ```

   In OpenSearch Dashboards, open the **OpenTelemetry** section and confirm that application logs and Istio access logs are arriving in the `logs-otel-v1-*` index.

6. **Delete the old Fluent Bit LogPipelines.** For the default setup:

   ```bash
   kubectl delete logpipeline sap-cloud-logging-application-logs
   kubectl delete logpipeline sap-cloud-logging-access-logs
   ```

   If you have additional Fluent Bit pipelines targeting CLS, delete those too.

7. **Update the Kyma dashboard ConfigMap**:

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/kyma-dashboard-configmap.yaml
   ```

8. **Switch to the OpenTelemetry predefined dashboards**: The predefined **Kyma** dashboards in SAP Cloud Logging are no longer populated. Use the **OpenTelemetry** section in OpenSearch Dashboards for application and Istio access logs going forward.

9. **Migrate custom dashboards**: If you have custom OpenSearch dashboards based on the `logs-json-kyma-*` or `logs-json-istio-envoy-kyma-*` indices, update them to use the `logs-otel-v1-*` index. Note that the log record structure also changed — the fields are now OTel semantic-convention attributes, so any field references in your visualizations need to be updated accordingly.
