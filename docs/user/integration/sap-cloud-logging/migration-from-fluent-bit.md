# Migrate from Fluent Bit to the OTel LogPipeline (SAP Cloud Logging)

If you have an existing Fluent Bit-based integration with SAP Cloud Logging, use this guide to migrate to the OTel LogPipeline. It consolidates your two Fluent Bit LogPipelines into one, simplifies secret management, and routes logs to the unified `logs-otel-v1-*` index used by the predefined **OpenTelemetry** dashboards in SAP Cloud Logging.

You can run the old and new pipelines in parallel during the migration. They write to separate indices and produce no duplicates, so the migration carries no risk of data loss or gaps in your logs.

## What Changes

| | Before (Fluent Bit) | After (OTel) |
|---|---|---|
| LogPipelines (application and access logs) | 2 with HTTP output | 1 with OTLP output |
| Secret keys | `ingest-mtls-endpoint`, `ingest-mtls-cert`, `ingest-mtls-key` | `ingest-otlp-endpoint`, `ingest-otlp-cert`, `ingest-otlp-key` |
| OpenSearch index (application logs) | `logs-json-kyma-*` | `logs-otel-v1-*` |
| OpenSearch index (Istio access logs) | `logs-json-istio-envoy-kyma-*` | `logs-otel-v1-*` |
| Istio extension provider | `stdout-json` | `kyma-logs` |
| Predefined SAP Cloud Logging dashboards | **Kyma** section | **OpenTelemetry** section |
| Kyma dashboard extension | Deep links use `ingest-mtls-*` secret keys | Deep links use `ingest-otlp-*` secret keys |

## Prerequisites

- You have an existing Fluent Bit-based SAP Cloud Logging integration with the `sap-cloud-logging-application-logs` and `sap-cloud-logging-access-logs` LogPipelines deployed.
- Kyma as the target deployment environment, with the following modules added (see [Quick Install](https://kyma-project.io/02-get-started/01-quick-install)):
  - Telemetry module
  - Istio module (default module)
  - SAP BTP Operator module (default module)
- You have permission to update your SAP Cloud Logging service instance parameters and create new service bindings.
<!-- markdown-link-check-disable -->
- Kubernetes CLI (kubectl) (see [Install the Kubernetes Command Line Tool](https://developers.sap.com/tutorials/cp-kyma-download-cli.html)).
<!-- markdown-link-check-enable -->
- UNIX shell or Windows Subsystem for Linux (WSL) to execute commands.

> [!WARNING]
> After enabling OTLP on your SAP Cloud Logging instance, you must delete the existing service binding and create a new one. The `ingest-otlp-*` secret keys are only present in bindings created **after** OTLP ingestion was enabled on the instance.

## Procedure

1. Enable OTLP ingestion on your existing SAP Cloud Logging instance by updating the service instance parameter:

   ```json
   {
     "ingest_otlp": {
       "enabled": true
     }
   }
   ```

   No new SAP Cloud Logging instance is needed. For details, see [Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters).

2. Delete the existing service binding and create a new one with the same name to get the `ingest-otlp-*` secret keys:

   ```bash
   kubectl delete servicebinding sap-cloud-logging -n sap-cloud-logging-integration
   kubectl apply -f - <<EOF
   apiVersion: services.cloud.sap.com/v1
   kind: ServiceBinding
   metadata:
     name: sap-cloud-logging
     namespace: sap-cloud-logging-integration
   spec:
     serviceInstanceName: sap-cloud-logging
     secretName: sap-cloud-logging
   EOF
   ```

   Wait until the binding status is `Created` before proceeding:

   ```bash
   kubectl get servicebinding sap-cloud-logging -n sap-cloud-logging-integration
   ```

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

   Verify the LogPipeline is healthy before proceeding:

   ```bash
   kubectl get logpipeline sap-cloud-logging
   ```

   Wait until the LogPipeline shows `Running` status.

4. Update your existing Istio Telemetry custom resource to add `kyma-logs` alongside `stdout-json`, so both providers emit access logs in parallel:

   ```yaml
   spec:
     accessLogging:
       - providers:
         - name: stdout-json
         - name: kyma-logs
   ```

   The `kyma-logs` provider emits access logs as OTel log records with semantic-convention attributes, which are compatible with the `logs-otel-v1-*` index. The `stdout-json` provider continues to feed the existing Fluent Bit LogPipeline during the migration. For details on Istio access log configuration, see [Collect Istio Access Logs](./../../collecting-logs/istio-support.md).

5. Confirm application logs and Istio access logs are arriving in the `logs-otel-v1-*` index by opening the **OpenTelemetry** section in OpenSearch Dashboards.

6. Update any saved objects and monitors that target the old Fluent Bit indices. If you imported saved objects or alerting monitors from the previous Fluent Bit-based integration guide, delete the old ones and re-import the current versions from [Set Up SAP Cloud Logging](./README.md).

   - **Saved objects** (dashboards, visualizations, index patterns): Replace index patterns targeting `logs-json-kyma-*` or `logs-json-istio-envoy-kyma-*` with `logs-otel-v1-*`. Review all field references — OTel log records use semantic-convention attributes that may differ structurally from the Fluent Bit JSON fields.
   - **Alerting monitors**: Replace index patterns the same way. Review all field-level conditions — the same attribute differences apply.

7. Delete the old Fluent Bit LogPipelines and remove `stdout-json` from the Istio Telemetry resource:

    > [!NOTE]
    > Only proceed if verification in step 5 succeeded. If logs are not arriving in the `logs-otel-v1-*` index, troubleshoot the new LogPipeline before removing the old ones.

    ```bash
    kubectl delete logpipeline sap-cloud-logging-application-logs
    kubectl delete logpipeline sap-cloud-logging-access-logs
    ```

    If you have additional Fluent Bit LogPipelines targeting SAP Cloud Logging, delete those too.

    Then update the Istio Telemetry resource to remove `stdout-json`:

    ```yaml
    spec:
      accessLogging:
        - providers:
          - name: kyma-logs
    ```

8. Update the Kyma dashboard ConfigMap:

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/kyma-dashboard-configmap.yaml
   ```

9. The **Kyma** section in OpenSearch Dashboards no longer receives data after the Fluent Bit pipelines are deleted. Use the **OpenTelemetry** section for application and Istio access logs going forward.

## Result

Your cluster sends all logs through a single OTel LogPipeline to SAP Cloud Logging. Logs appear in the `logs-otel-v1-*` index, and the predefined **OpenTelemetry** dashboards in SAP Cloud Logging are populated. The legacy Fluent Bit LogPipelines, their `ingest-mtls-*` credentials, and the old Fluent Bit-based dashboards and monitors are no longer in use.

## Related Information

- [Set Up SAP Cloud Logging](./README.md)
- [Collect Istio Access Logs](./../../collecting-logs/istio-support.md)
- [SAP Cloud Logging Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters)
