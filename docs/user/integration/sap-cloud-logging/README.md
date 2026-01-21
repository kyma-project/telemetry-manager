# Integrate with SAP Cloud Logging

| Category     |                       |
| ------------ | --------------------- |
| Signal types | logs, traces, metrics |
| Backend type | third-party remote    |
| OTLP-native  | yes                   |

Configure the Telemetry module to send logs, metrics, and traces from your cluster to an SAP Cloud Logging instance. By centralizing this data in your SAP Cloud Logging instance, you can store, visualize, and analyze the observability of your applications.

## Table of Content

- [Prerequisites](#prerequisites)
- [Context](#context)
- [Ship Logs to SAP Cloud Logging](#ship-logs-to-sap-cloud-logging)
- [Ship Distributed Traces to SAP Cloud Logging](#ship-traces-to-sap-cloud-logging)
- [Ship Metrics to SAP Cloud Logging](#ship-metrics-to-sap-cloud-logging)
- [Set Up Kyma Dashboard Integration](#set-up-kyma-dashboard-integration)
- [Use SAP Cloud Logging Alerts](#use-sap-cloud-logging-alerts)
- [Use SAP Cloud Logging Dashboards](#use-sap-cloud-logging-dashboards)

## Prerequisites

- Kyma as the target deployment environment, with the following modules added (see [Quick Install](https://kyma-project.io/#/02-get-started/01-quick-install)):
  - Telemetry module
  - To collect data from your Istio service mesh: Istio module (default module)
  - SAP BTP Operator module (default module)
- An instance of [SAP Cloud Logging](https://help.sap.com/docs/cloud-logging?locale=en-US&version=Cloud) with OpenTelemetry ingestion enabled. For details, see [Ingest via OpenTelemetry API Endpoint](https://help.sap.com/docs/SAP_CLOUD_LOGGING/d82d23dc499c44079e1e779c1d3a5191/fdc78af7c69246bc87315d90a061b321.html?locale=en-US).
  > [!TIP]
  > Create the SAP Cloud Logging instance with the SAP BTP service operator (see [Create an SAP Cloud Logging Instance through SAP BTP Service Operator](https://help.sap.com/docs/cloud-logging/cloud-logging/create-sap-cloud-logging-instance-through-sap-btp-service-operator?locale=en-US&version=Cloud)), because it takes care of creation and rotation of the required Secret. However, you can choose any other method of creating the instance and the Secret, as long as the parameter for OTLP ingestion is enabled in the instance. For details, see [Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters?locale=en-US&version=Cloud).
- A Secret in the respective namespace in your Kyma cluster, holding the credentials and endpoints for the instance. It’s recommended that you rotate your Secret (see [SAP BTP Security Recommendation BTP-CLS-0003](https://help.sap.com/docs/btp/sap-btp-security-recommendations-c8a9bb59fe624f0981efa0eff2497d7d/sap-btp-security-recommendations?seclist-index=BTP-CLS-0003&version=Cloud&locale=en-US)). In the following example, the Secret is named `sap-cloud-logging` and the namespace `sap-cloud-logging-integration`, as illustrated in the [secret-example.yaml](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/integration/sap-cloud-logging/secret-example.yaml).
<!-- markdown-link-check-disable -->
- Kubernetes CLI (kubectl) (see [Install the Kubernetes Command Line Tool](https://developers.sap.com/tutorials/cp-kyma-download-cli.html)).
<!-- markdown-link-check-enable -->
- UNIX shell or Windows Subsystem for Linux (WSL) to execute commands.

## Context

The Telemetry module supports shipping logs and ingesting distributed traces as well as metrics from applications and the Istio service mesh to SAP Cloud Logging.

First, set up the Telemetry module to ship the logs, traces, and metrics to your backend by deploying the pipelines and other required resources. Then, you configure Kyma dashboard integration. Finally, set up SAP Cloud Logging alerts and dashboards.

SAP Cloud Logging is an instance-based and environment-agnostic observability service to store, visualize, and analyze logs, metrics, and traces.

![setup](./../assets/sap-cloud-logging.drawio.svg)

## Ship Logs to SAP Cloud Logging

You can set up ingestion of logs from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.

### Procedure

#### Set Up Application Logs
<!-- using HTML so it's collapsed in GitHub, don't switch to docsify tabs -->
1. Deploy a [LogPipeline](./../../collecting-logs/README.md) for application logs:

   - For OTLP, run:
    <div tabs name="logs">
      <details><summary>Script: Application Logs</summary>

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

      </details>
    </div>

1. Verify that the LogPipeline is running:

  ```bash
  kubectl get logpipelines
  ```

#### Set Up Istio Access Logs

By default, Istio sidecar injection and Istio access logs are disabled in Kyma. To analyze them, you must enable them:

1. Enable Istio sidecar injection for your workload (see [Enabling Istio Sidecar Proxy Injection](https://kyma-project.io/#/istio/user/tutorials/01-40-enable-sidecar-injection)).

1. Configure the [Istio](https://istio.io/latest/docs/reference/config/telemetry/) Telemetry resource (see [Configure Istio Access Logs](./../../collecting-logs/istio-support.md)) with the `kyma-logs` extension provider.

## Ship Traces to SAP Cloud Logging

You can set up ingestion of distributed traces from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.

### Procedure

#### Set Up Traces

1. Deploy a [TracePipeline](./../../collecting-traces/README.md):

   <div tabs name="distributedtraces">
     <details><summary>Script: Distributed Traces</summary>

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: telemetry.kyma-project.io/v1beta1
    kind: TracePipeline
    metadata:
      name: sap-cloud-logging
    spec:
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

     </details>
   </div>

1. Verify that the TracePipeline is running:

  ```bash
  kubectl get tracepipelines
  ```

#### Set Up Istio Tracing

By default, Istio sidecar injection and Istio tracing are disabled in Kyma. To analyze them, you must enable them:

1. Enable Istio sidecar injection for your workload (see [Enabling Istio Sidecar Proxy Injection](https://kyma-project.io/#/istio/user/tutorials/01-40-enable-sidecar-injection)).
1. Configure the [Istio](https://istio.io/latest/docs/reference/config/telemetry/) Telemetry resource to use the kyma-traces extension provider based on OTLP (see [Configure Istio Tracing](./../../collecting-traces/istio-support.md)).

## Ship Metrics to SAP Cloud Logging

You can set up ingestion of metrics from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.

### Procedure

1. Deploy a [MetricPipeline](./../../collecting-metrics/README.md):

   <div tabs name="SAPCloudLogging">
     <details><summary>Script: SAP Cloud Logging</summary>

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: telemetry.kyma-project.io/v1beta1
    kind: MetricPipeline
    metadata:
      name: sap-cloud-logging
    spec:
      input:
        prometheus:
          enabled: false
        istio:
          enabled: false
        runtime:
          enabled: false
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

     </details>
   </div>

    The default configuration creates a gateway to receive OTLP metrics from your applications.
1. Optional: To collect additional metrics, such as those from the runtime or Istio, configure the presets in the input section of the MetricPipeline.
For the available options, see [Configure Metrics Collection](./../../collecting-metrics/README.md).
1. Verify that the MetricPipeline is running:

  ```bash
  kubectl get metricpipelines
  ```

## Set Up Kyma Dashboard Integration

You can add direct links from Kyma dashboard to SAP Cloud Logging.

### Procedure

1. If your Secret has a name or namespace different from the example, download the file and edit the dataSources section before you apply it.
1. Apply the ConfigMap:

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/kyma-dashboard-configmap.yaml
   ```

## Use SAP Cloud Logging Alerts

You can import predefined alerts for SAP Cloud Logging to monitor the health of your telemetry integration.

### Procedure

1. In the SAP Cloud Logging dashboard, define a “notification channel” to receive alert notifications.
1. To import a monitor, use the development tools of the SAP Cloud Logging dashboard.
1. Execute `POST _plugins/_alerting/monitors`, followed by the contents of the respective JSON file.
1. Depending on the pipelines you are using, enable some or all of the following alerts:
   The alerts are based on JSON documents defining a Monitor for the alerting plugin.

<!-- markdown-link-check-disable -->
   | Monitored Component | File | Description |
   | -- | -- | -- |
   | SAP Cloud Logging | [alert-health.json](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-health.json) | Monitors the health of the underlying OpenSearch cluster in SAP Cloud Logging using the [cluster health API](https://opensearch.org/docs/1.3/api-reference/cluster-api/cluster-health). Triggers if the cluster status becomes `red`. |
   | SAP Cloud Logging | [alert-rejection-in-progress.json](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-rejection-in-progress.json) | Monitors the `cls-rejected-*` index for new data. Triggers if new rejected data is observed. |
   | Kyma Telemetry Integration | [alert-telemetry-status.json](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-app-log-ingestion.json) | Monitors the status of the Telemetry module. Triggers if the module reports a non-ready state. |
   | Kyma Telemetry Integration | [alert-log-ingestion.json](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-log-ingestion.json) | Monitors the LogPipeline. Triggers if log data stops flowing to SAP Cloud Logging. |
   | Kyma Telemetry Integration | [alert-trace-ingestion.json](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-trace-ingestion.json) | Monitors the TracePipeline. Triggers if trace data stops flowing to SAP Cloud Logging. |
   | Kyma Telemetry Integration | [alert-metric-ingestion.json](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-metric-ingestion.json) | Monitors the MetricPipeline. Triggers if metric data stops flowing to SAP Cloud Logging. |

<!-- markdown-link-check-enable -->
5. After importing, edit the monitor to attach your notification channel or destination and adjust thresholds as needed.
6. Verify that the new monitor definition is listed among the SAP Cloud Logging alerts.

## Use SAP Cloud Logging Dashboards

You can view logs, traces, and metrics in SAP Cloud Logging dashboards. Several dashboards come with SAP Cloud Logging, and you can import additional dashboards as needed.

### Procedure

<!-- markdown-link-check-disable -->
- For the status of the SAP Cloud Logging integration with the Telemetry module, import the file [dashboard-status.ndjson](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-status.ndjson).
- For application logs and Istio access logs: Use the preconfigured dashboard `Overview`.
- For traces, use the OpenSearch plugin “Observability”.
- For runtime metrics, import the file [dashboard-runtime.ndjson](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-runtime.ndjson).
- For Istio Pod metrics, import the file [dashboard-istio.ndjson](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-istio.ndjson).
<!-- markdown-link-check-enable -->
