# Integrate with SAP Cloud Logging

## Overview

| Category| |
| - | - |
| Signal types | logs, traces, metrics |
| Backend type | third-party remote |
| OTLP-native | yes for traces and metrics, no for logs |

Learn how to define LogPipelines and TracePipelines to ingest application and access logs as well as distributed trace data in instances of [SAP Cloud Logging](https://help.sap.com/docs/cloud-logging?locale=en-US&version=Cloud).

SAP Cloud Logging is an instance-based and environment-agnostic observability service that builds upon OpenSearch to store, visualize, and analyze logs, metrics, and traces.

![setup](./../assets/sap-cloud-logging.drawio.svg)

## Table of Content

- [Integrate with SAP Cloud Logging](#integrate-with-sap-cloud-logging)
  - [Overview](#overview)
  - [Table of Content](#table-of-content)
  - [Prerequisites](#prerequisites)
  - [Ship Logs to SAP Cloud Logging](#ship-logs-to-sap-cloud-logging)
    - [Set Up Application Logs](#set-up-application-logs)
    - [Set Up Access Logs](#set-up-access-logs)
  - [Ship Distributed Traces to SAP Cloud Logging](#ship-distributed-traces-to-sap-cloud-logging)
  - [Ship Metrics to SAP Cloud Logging](#ship-metrics-to-sap-cloud-logging)
  - [Kyma Dashboard Integration](#kyma-dashboard-integration)
  - [SAP Cloud Logging Alerts](#sap-cloud-logging-alerts)
    - [Import Alerts](#import-alerts)
    - [Recommended Alerts](#recommended-alerts)
  - [SAP Cloud Logging Dashboards](#sap-cloud-logging-dashboards)

## Prerequisites

- Kyma as the target deployment environment.
- The [Telemetry module](../../README.md) is [added](https://kyma-project.io/#/02-get-started/01-quick-install).
- If you want to use Istio access logs, make sure that the [Istio module](https://kyma-project.io/#/istio/user/README) is added.
- An instance of SAP Cloud Logging with OpenTelemetry enabled to ingest distributed traces.
  > [!TIP]
  > It's recommended to create the instance with the SAP BTP service operator (see [Create an SAP Cloud Logging Instance through SAP BTP Service Operator](https://help.sap.com/docs/cloud-logging/cloud-logging/create-sap-cloud-logging-instance-through-sap-btp-service-operator?locale=en-US&version=Cloud)), because it takes care of creation and rotation of the required Secret. However, you can choose any other method of creating the instance and the Secret, as long as the parameter for OTLP ingestion is enabled in the instance. For details, see [Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters?locale=en-US&version=Cloud).
- A Secret in the respective namespace in the Kyma cluster, holding the credentials and endpoints for the instance. In this guide, the Secret is named `sap-cloud-logging` and the namespace `sap-cloud-logging-integration` as illustrated in this [example](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/integration/sap-cloud-logging/secret-example.yaml).
<!-- markdown-link-check-disable -->
- Kubernetes CLI (kubectl) (see [Install the Kubernetes Command Line Tool](https://developers.sap.com/tutorials/cp-kyma-download-cli.html)).
<!-- markdown-link-check-enable -->
- UNIX shell or Windows Subsystem for Linux (WSL) to execute commands.

## Ship Logs to SAP Cloud Logging

The Telemetry module supports the convenient shipment of applications and access logs using LogPipeline custom resources. For more details, see [Kyma Telemetry Application Logs Documentation](./../../02-logs.md). The setup distinguishes application logs and access logs, which can be configured independently.

### Set Up Application Logs
<!-- using HTML so it's collapsed in GitHub, don't switch to docsify tabs -->
1. Deploy the LogPipeline for application logs:

   <div tabs name="applicationlogs">
     <details><summary>Script: Application Logs</summary>

    ```bash
    kubectl apply -n sap-cloud-logging-integration -f - <<EOF
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: LogPipeline
    metadata:
      name: sap-cloud-logging-application-logs
    spec:
      input:
        application:
          containers:
            exclude:
              - istio-proxy
      output:
        http:
          dedot: true
          host:
            valueFrom:
              secretKeyRef:
                name: sap-cloud-logging
                namespace: sap-cloud-logging-integration
                key: ingest-mtls-endpoint
          tls:
            cert:
              valueFrom:
                secretKeyRef:
                  name: sap-cloud-logging
                  namespace: sap-cloud-logging-integration
                  key: ingest-mtls-cert
            key:
              valueFrom:
                secretKeyRef:
                  name: sap-cloud-logging
                  namespace: sap-cloud-logging-integration
                  key: ingest-mtls-key
          uri: /customindex/kyma
    EOF
    ```
      </details>
    </div>

2. Wait for the LogPipeline to be in the `Running` state. To check the state, run:

    ```bash
    kubectl get logpipelines
    ```

### Set Up Access Logs

By default, Istio sidecar injection and Istio access logs are disabled in Kyma.

To analyze access logs of your workload in the default SAP Cloud Logging dashboards shipped for SAP BTP, Kyma runtime, you must enable them:

1. Enable Istio sidecar injection for your workload following [Enabling Istio Sidecar Injection](https://kyma-project.io/#/istio/user/operation-guides/02-20-enable-sidecar-injection)

1. Enable Istio access logs for your workload following [Enable Istio access logs](https://kyma-project.io/#/istio/user/operation-guides/02-30-enable-istio-access-logs).

   > [!WARNING]
   > The provided feature uses an Istio API in the alpha state, which may or may not be continued in future releases.

2. Deploy the LogPipeline for Istio access logs and enable access logs in Kyma:

   <div tabs name="accesslogs">
     <details><summary>Script: Access Logs</summary>

    ```bash
    kubectl apply -n sap-cloud-logging-integration -f - <<EOF
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: LogPipeline
    metadata:
      name: sap-cloud-logging-access-logs
    spec:
      input:
        application:
          containers:
            include:
              - istio-proxy
      output:
        http:
          dedot: true
          host:
            valueFrom:
              secretKeyRef:
                name: sap-cloud-logging
                namespace: sap-cloud-logging-integration
                key: ingest-mtls-endpoint
          tls:
            cert:
              valueFrom:
                secretKeyRef:
                  name: sap-cloud-logging
                  namespace: sap-cloud-logging-integration
                  key: ingest-mtls-cert
            key:
              valueFrom:
                secretKeyRef:
                  name: sap-cloud-logging
                  namespace: sap-cloud-logging-integration
                  key: ingest-mtls-key
          uri: /customindex/istio-envoy-kyma
    EOF
    ```
      </details>
    </div>

3. Wait for the LogPipeline to be in the `Running` state. To check the state, run:

    ```bash
    kubectl get logpipelines
    ```

## Ship Distributed Traces to SAP Cloud Logging

The Telemetry module supports ingesting [distributed traces](./../../03-traces.md) from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.
To enable shipping traces to the SAP Cloud Logging service instance, follow this procedure:

1. Deploy the Istio Telemetry resource:

   <div tabs name="istiotraces">
     <details><summary>Script: Istio Traces</summary>

   ```bash
    kubectl apply -n istio-system -f - <<EOF
    apiVersion: telemetry.istio.io/v1alpha1
    kind: Telemetry
    metadata:
      name: tracing-default
    spec:
      tracing:
      - providers:
        - name: "kyma-traces"
        randomSamplingPercentage: 1.0
    EOF
    ```
     </details>
   </div>

   The default configuration has the **randomSamplingPercentage** property set to `1.0`, meaning it samples 1% of all requests. To change the sampling rate, adjust the property to the desired value, up to 100 percent.

   > [!WARNING]
   > Be cautious when you configure the **randomSamplingPercentage**:
   > - Traces might consume a significant storage volume in Cloud Logging Service.
   > - The Kyma trace collector component does not scale automatically.

1. Deploy the TracePipeline:

   <div tabs name="distributedtraces">
     <details><summary>Script: Distributed Traces</summary>

   ```bash
    kubectl apply -n sap-cloud-logging-integration -f - <<EOF
    apiVersion: telemetry.kyma-project.io/v1alpha1
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

1. Wait for the TracePipeline to be in the `Running` state. To check the state, run:

   ```bash
   kubectl get tracepipelines
   ```

## Ship Metrics to SAP Cloud Logging

The Telemetry module supports ingesting [metrics](./../../04-metrics.md) from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.
To enable shipping traces to the SAP Cloud Logging service instance, follow this procedure:

1. Deploy the MetricPipeline:

   <div tabs name="SAPCloudLogging">
     <details><summary>Script: SAP Cloud Logging</summary>

   ```bash
    kubectl apply -n sap-cloud-logging-integration -f - <<EOF
    apiVersion: telemetry.kyma-project.io/v1alpha1
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

    By default, the MetricPipeline assures that a gateway is running in the cluster to push OTLP metrics.

1. If you want to use additional metric collection, configure the presets under `input`. For the available options, see [Metrics](./../../04-metrics.md).

1. Wait for the MetricPipeline to be in the `Running` state. To check the state, run:

   ```bash
   kubectl get metricpipelines
   ```

## Kyma Dashboard Integration

For easier access, add a navigation node to the Observability section as well as deep links to the Pod, Deployment, and Namespace views of the Kyma dashboard.

1. Read the Cloud Logging dashboard URL from the secret:

    ```bash
    export DASHBOARD_URL=$(kubectl -n sap-cloud-logging-integration get secret sap-cloud-logging --template='{{index .data "dashboards-endpoint" | base64decode}}')
    ```

1. Download the two configmaps containing the exemplaric configuration:

    ```bash
    curl -o configmap-navigation.yaml https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/configmap-navigation.yaml
    curl -o configmap-deeplinks.yaml https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/configmap-deeplinks.yaml
    ```

1. Replace placeholders in the configmaps with the URL:

    ```bash
    sed -e "s/{PLACEHOLDER}/$DASHBOARD_URL/" configmap-navigation.yaml
    sed -e "s/{PLACEHOLDER}/$DASHBOARD_URL/" configmap-deeplinks.yaml
    ```

1. Apply the configmaps:

    ```bash
    kubectl apply -f configmap-navigation.yaml
    kubectl apply -f configmap-deeplinks.yaml
    ```

## SAP Cloud Logging Alerts

SAP Cloud Logging provides an alerting mechanism based on the OpenSearch Dashboard's [alerting plugin](https://opensearch.org/docs/1.3/observing-your-data/alerting/index/). Learn how to define and import recommended alerts:

### Import Alerts

The following alerts are based on JSON documents defining a `Monitor` for the alerting plugin, which works instantly after import. However, you must manually add a `destination` to the configured notification action. Also, adjust the intervals and thresholds to your specific needs.

1. To import a monitor, go to `Management > Dev Tools` in the Cloud Logging Dashboard.
2. Execute `POST _plugins/_alerting/monitors`, followed by the contents of the respective JSON content.
3. Verify that the new monitor definition is listed at `OpenSearch Plugins > Alerting`.

### Recommended Alerts

Depending on the pipelines you are using, enable the some or all of the following alerts:

| Category | File | Description |
| -- | -- | -- |
| Cloud Logging | [OpenSearch cluster health](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-health.json) | The OpenSearch cluster might become unhealthy, which is indicated by a "red" status using the [cluster health API](https://opensearch.org/docs/1.3/api-reference/cluster-api/cluster-health).|
| Kyma Telemetry Integration | [Application log ingestion](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-app-log-ingestion.json) | The LogPipeline for shipping [application logs](#ship-logs-to-sap-cloud-logging) might lose connectivity to SAP Cloud Logging, with the effect that no application logs are ingested anymore.|
| Kyma Telemetry Integration | [Access log ingestion](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-access-log-ingestion.json) | The LogPipeline for shipping [access logs](#ship-logs-to-sap-cloud-logging) might lose connectivity to SAP Cloud Logging, with the effect that no access logs are ingested anymore. |
| Kyma Telemetry Integration | [Trace ingestion](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-trace-ingestion.json) | The TracePipeline for shipping [traces](#ship-distributed-traces-to-sap-cloud-logging) might lose connectivity to SAP Cloud Logging, with the effect that no traces are ingested anymore. |
| Kyma Telemetry Integration | [Metric ingestion](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/alert-metric-ingestion.json) | The MetricPipeline for shipping [metrics](#ship-metrics-to-sap-cloud-logging) might lose connectivity to SAP Cloud Logging, with the effect that no metrics are ingested anymore. |

## SAP Cloud Logging Dashboards

SAP Cloud Logging provides an extensive set of dashboards under `OpenSearch Dashboards > Dashboard`, which give insights in traffic and application logs. These dashboards are prefixed with `Kyma_`, and are based on both kinds of [log ingestion](#ship-logs-to-sap-cloud-logging): application and access logs.

Additionally, you can view distributed traces under `OpenSearch Plugins > Observability`.

To view the metrics generated by the inputs of a MetricPipeline in a Dashboard, you can manually upload them. In the SAP Cloud Logging Dashboard, go to **Stack Management > Saved Objects** and import the following files:

| Category | File | Description |
| -- | -- | -- |
| Runtime Metrics | [Kyma Container Metrics](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-runtime.ndjson) | Visualizes the container- and Pod-related metrics collected by the MetricPipeline `runtime` input.|
| Istio Metrics | [Kyma Istio Service Metrics](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-istio.ndjson) | Visualizes the Istio metrics of Pods that have an active Istio sidecar injection. The metrics are collected by the MetricPipeline `istio` input. |
