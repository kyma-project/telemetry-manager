# Integrate with SAP Cloud Logging service

The SAP Cloud Logging service is an instance-based and environment-agnostic observability service that builds upon OpenSearch to store, visualize, and analyze logs, metrics, and traces. This guide explains how to define LogPipelines and TracePipelines in order to ingest application and access logs as well as distributed trace data in the SAP Cloud Logging service.

## Prerequisites

- A SAP Cloud Logging service with OpenTelemetry enabled to ingest distributed traces
- A secret named `cls` in `cls-integration` namespace, holding the credentials and endpoints for the instance

## Ship Logs to Cloud Logging Service

The telemetry module supports convenient shipment of application and access logs using LogPipeline custom resources (see [Kyma Telemetry Application Logs Documentation](./../../02-logs)). The setup distinguishes application logs and access logs which can be configured independently.
To enable shipping logs to Cloud Logging Service follow the below procedure:

1. Deploy the LogPipeline for application logs:
    ```
    cat <<EOF | kubectl apply -n cls-integration -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: LogPipeline
    metadata:
      name: cls-application-logs
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
                name: cls
                namespace: cls-integration
                key: ingest-mtls-endpoint
          tls:
            cert:
              valueFrom:
                secretKeyRef:
                  name: cls
                  namespace: cls-integration
                  key: ingest-mtls-cert
            key:
              valueFrom:
                secretKeyRef:
                  name: cls
                  namespace: cls-integration
                  key: ingest-mtls-key
          uri: /customindex/kyma
    ```
1. Deploy the LogPipeline for Istio access logs and enable access logs in Kyma:
    ```
    cat <<EOF | kubectl apply -n cls-integration -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: LogPipeline
    metadata:
      name: cls-access-logs
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
                name: cls
                namespace: cls-integration
                key: ingest-mtls-endpoint
          tls:
            cert:
              valueFrom:
                secretKeyRef:
                  name: cls
                  namespace: cls-integration
                  key: ingest-mtls-cert
            key:
              valueFrom:
                secretKeyRef:
                  name: cls
                  namespace: cls-integration
                  key: ingest-mtls-key
          uri: /customindex/istio-envoy-kyma
    ```
   Kyma sets Istio access logs to disabled by default. To enable Istio access logs selectively for your workload, follow the [access logs guide](https://kyma-project.io/docs/kyma/main/04-operation-guides/operations/obsv-03-enable-istio-access-logs/).
   As a result, access logs can be analyzed in the default dashboards shipped for the SAP BTP, Kyma runtime.

   >**CAUTION:** The provided feature uses an Istio API in alpha state, which may or may not be continued in future releases.

1. Wait for the LogPipeline Kubernetes objects to be in Running state:
    ```
    kubectl get logpipelines
    ```

## Ship Distributed Traces to BTP Cloud Logging Service

The telemetry module supports to ingest [distributed traces](./../../03-traces/) from applications and the Istio service mesh to the OTLP endpoint of SAP Cloud Logging service.
To enable shipping traces to the SAP Cloud Logging Service, follow the below procedure:

1. Deploy the Istio Telemetry resource by executing the following command:
    ```
    cat <<EOF | kubectl apply -n istio-system -f -
    apiVersion: telemetry.istio.io/v1alpha1
    kind: Telemetry
    metadata:
      name: tracing-default
    spec:
      tracing:
      - providers:
        - name: "kyma-traces"
        randomSamplingPercentage: 1.0
    ```
    The default configuration has the **randomSamplingPercentage** property set to `1.0`, meaning it samples 1% of all requests. To change the sampling rate, adjust the property to the desired value up to 100 percent.
    > **Note:**
    > Be mindful of configuring the randomSamplingPercentage because
    >  - traces might consume a significant storage volume in Cloud Logging Service
    >  - the Kyma trace collector component does not scale automatically.

2. Deploy the TracePipeline by executing the following command:
    ```
    cat <<EOF | kubectl apply -n cls-integration -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: TracePipeline
    metadata:
      name: cls
    spec:
      output:
        otlp:
          endpoint:
            valueFrom:
              secretKeyRef:
                name: cls
                namespace: cls-integration
                key: ingest-otlp-endpoint
          tls:
            cert:
              valueFrom:
                secretKeyRef:
                  name: cls
                  namespace: cls-integration
                  key: ingest-otlp-cert
            key:
              valueFrom:
                secretKeyRef:
                  name: cls
                  namespace: cls-integration
                  key: ingest-otlp-key   
    ```

3. Wait for the TracePipeline to be in Running state:
    ```
    kubectl get tracepipelines
    ```