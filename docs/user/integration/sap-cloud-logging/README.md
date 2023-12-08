# Integrate with SAP Cloud Logging

## Overview

Learn how to define LogPipelines and TracePipelines to ingest application and access logs as well as distributed trace data in instances of [SAP Cloud Logging](https://help.sap.com/docs/cloud-logging?locale=en-US&version=Cloud).

SAP Cloud Logging is an instance-based and environment-agnostic observability service that builds upon OpenSearch to store, visualize, and analyze logs, metrics, and traces.

- Signal type: logs, metrics, and traces
- Backend type: third-party remote
- OTLP-native: yes

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](../README.md) is [enabled](https://kyma-project.io/#/02-get-started/08-install-uninstall-upgrade-kyma-module?id=install-uninstall-and-upgrade-kyma-with-a-module)
- An instance of SAP Cloud Logging with OpenTelemetry enabled to ingest distributed traces.
  >**TIP:** It's recommended to create it with the SAP BTP Service Operator (see [Create an SAP Cloud Logging Instance through SAP BTP Service Operator](https://help.sap.com/docs/cloud-logging/cloud-logging/create-sap-cloud-logging-instance-through-sap-btp-service-operator?locale=en-US&version=Cloud)), because it takes care of creation and rotation of the required Secret. However, you can choose any other method of creating the instance and the Secret, as long as you make sure that OTLP ingestion is enabled [see Configuration Parameters](https://help.sap.com/docs/cloud-logging/cloud-logging/configuration-parameters?locale=en-US&version=Cloud) in the instance.
- A Secret in the respective namespace in the Kyma cluster, holding the credentials and endpoints for the instance. In this example, the Secret is named `sap-cloud-logging` and the namespace  `sap-cloud-logging-integration`.
- Kubernetes CLI (kubectl) (see [Install the Kubernetes Command Line Tool](https://developers.sap.com/tutorials/cp-kyma-download-cli.html))
- UNIX shell or Windows Subsystem for Linux (WSL) to execute commands

## Ship Logs to SAP Cloud Logging

The Telemetry module supports the convenient shipment of applications and access logs using LogPipeline custom resources. For more details, see [Kyma Telemetry Application Logs Documentation](./../../02-logs.md). The setup distinguishes application logs and access logs, which can be configured independently.
To enable log shipment to the SAP Cloud Logging service instance, follow the this procedure:

1. Deploy the LogPipeline for application logs:

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

1. Deploy the LogPipeline for Istio access logs and enable access logs in Kyma:

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

   By default, Kyma sets Istio access logs to disabled. To enable Istio access logs selectively for your workload, follow [Enable Istio access logs](https://kyma-project.io/#/istio/user/02-operation-guides/operations/02-30-enable-istio-access-logs).
   As a result, access logs can be analyzed in the default dashboards shipped for SAP BTP, Kyma runtime.

   >**CAUTION:** The provided feature uses an Istio API in the alpha state, which may or may not be continued in future releases.

1. Wait for the LogPipeline to be in the `Running` state. To check the state, run:

    ```bash
    kubectl get logpipelines
    ```

## Ship Distributed Traces to SAP Cloud Logging

The Telemetry module supports ingesting [distributed traces](./../../03-traces.md) from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.
To enable shipping traces to the SAP Cloud Logging service instance, follow the this procedure:

1. Deploy the Istio Telemetry resource:

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

    The default configuration has the **randomSamplingPercentage** property set to `1.0`, meaning it samples 1% of all requests. To change the sampling rate, adjust the property to the desired value, up to 100 percent.

    > **CAUTION:** Be cautious when you configure the **randomSamplingPercentage**:
    > - Traces might consume a significant storage volume in Cloud Logging Service.
    > - The Kyma trace collector component does not scale automatically.

1. Deploy the TracePipeline:

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

1. Wait for the TracePipeline to be in the `Running` state. To check the state, run:

   ```bash
   kubectl get tracepipelines
   ```

## Ship Metrics to SAP Cloud Logging (experimental)

The Telemetry module supports ingesting [metrics](./../../04-metrics.md) from applications and the Istio service mesh to the OTLP endpoint of the SAP Cloud Logging service instance.
To enable shipping traces to the SAP Cloud Logging service instance, follow the this procedure:

1. Deploy the MetricPipeline:

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

    By default, the MetricPipeline will assure that a gateway is running in the cluster to push OTLP metrics. Additionally, the `input` allows you to enable different presets which enables additional metric collections. See the [metrics documentation](./../../04-metrics.md) for the available options.

1. Wait for the MetricPipeline to be in the `Running` state. To check the state, run:

   ```bash
   kubectl get metricpipelines
   ```