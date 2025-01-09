# Integrate Custom Instrumentation App

## Overview

| Category| |
| - | - |
| Signal types | traces, metrics, logs |
| Backend type | custom in-cluster, third-party remote |
| OTLP-native | yes |

Learn how to instrument your custom Golang application using the [OTel SDK](https://opentelemetry.io/docs/languages/) and exporting metrics and trace data using the OTel SDK. The sample application will be configured to push trace and metric data using OTLP to the collector that's provided by Kyma, so that they are collected together with the related Istio trace data.

For examples using the OTel-SDK in a different langue, please refer to the official [OTel guides](https://opentelemetry.io/docs/languages/) and the [OTel demo app](./../opentelemetry-demo/).

![setup](./../assets/sample-app.drawio.svg)

## Prerequisites

- Kyma as the target deployment environment.
- The [Telemetry module](../../README.md) is [added](https://kyma-project.io/#/02-get-started/01-quick-install)
- [Kubectl version that is within one minor version (older or newer) of `kube-apiserver`](https://kubernetes.io/releases/version-skew-policy/#kubectl)

## Exploring the Sample App

The sample app is a small webserver written in Golang, which exposes to endpoints `forward` and `terminate`. When calling the endpoints via HTTP, metrics are getting counted up and spans are emmitted using OTLP exporters. Furthermore, structured logs are written to `stdout`.
<!-- markdown-link-check-disable-next-line -->
The application is located in the [`telemetry-manager`](https://github.com/kyma-project/telemetry-manager) repo in the folder [`docs/user/integration/sample-app`](https://github.com/kyma-project/telemetry-manager/tree/main/docs/user/integration/sample-app).

### Setup

The application consists of following go files:
- `main.go`
    - the main method with the actual handler routines
    - the initialization of the tracer and meter provider including the metric definitions
    - auto-instrumentation of the handler routines using the OTEL-SDK
- `setup.go`
    - the setup of the tracer and meter providers using the OTel SDK
    - configures either OTLP GRPC exporters or a console exporter

### Metrics

- The `init` method defines the available metrics
  - `cpu.temperature.celsius` is a Gauge which gets updated constantly via an observable
  - `hd.errors.total` is a Counter which gets increased on every `terminate` handler call
  - `cpu.energy.watt` is a Histogram which gets increased in one bucket on every `terminate` handler call
- The `main` method initializes and registers the meterProvider
- In the `main` method, the handler functions are getting auto-instrumented using the [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) library, having request metrics auto-instrumented starting with `http.server.request`

### Traces

- The `main` method initializes a tracer and a propagator
- In the handler routines the tracer is used to create new spans
- The propagation in the `forward` method is done automatically by passing the request context to the upstream call
- In the `main` method, the handler functions are getting auto-instrumented using the [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) library, having request spans auto-instrumented having span names prefixed with `auto-`

### Logs

The `main.go` initializes the Golang `slog` logger, which is consistently used for all application logs. It is configured to print in JSON and is available the logs will add the traceId attribute.

### Running local

By default the exporter are configured to print to stdout, so that you can run the app from local.

1. Checkout the `telemetry-manager` repo and browse into the folder `docs/user/integration/sample-app`
1. Build and start the application:
    ```sh
    make run
    ```

## Deploying the Sample App

### Activate Kyma Telemetry with a backend
1. Provide a tracing backend and activate it.
   Install [Jaeger in-cluster](../jaeger/README.md) or provide a custom backend supporting the OTLP protocol (like [SAP Cloud Logging](./../sap-cloud-logging/)).
1. Provide a metric backend and activate it.
   <!-- markdown-link-check-disable-next-line -->
   Install [Prometheus in-cluster](../prometheus/README.md) or provide a custom backend supporting the OTLP protocol (like [SAP Cloud Logging](./../sap-cloud-logging/)).
1. Provide a log backend and activate it.
   Install [Loki in-cluster](../loki/README.md) or provide a custom backend supporting the OTLP protocol (like [SAP Cloud Logging](./../sap-cloud-logging/)).

### Deploy the sample application

1. Export your Namespace as a variable. Replace the `{namespace}` placeholder in the following command and run it:

    ```bash
    export K8S_SAMPLE_NAMESPACE="{namespace}"
    ```

1. Ensure that your Namespace has Istio sidecar injection enabled to have a secure communication enabled by default:

   ```bash
   kubectl label namespace ${K8S_SAMPLE_NAMESPACE} istio-injection=enabled
   ```

1. Deploy the service using the prepared Deployment manifest and image:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/docs/user/integration/sample-app/deployment/deployment.yaml -n $K8S_SAMPLE_NAMESPACE
    ```

1. Verify the application:

   Port-forward to the service:
   ```sh
   kubectl -n $K8S_SAMPLE_NAMESPACE port-forward svc/sample-app 8080
   ```
   and call the forward endpoint:
   ```sh
   curl http://localhost:8080/forward
   ```

### Cleanup

Run the following commands to completely remove the sample app from the cluster:

2. Run the following command to completely remove the example service and all its resources from the cluster:

    ```bash
    kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/docs/user/integration/sample-app/deployment/deployment.yaml -n $K8S_SAMPLE_NAMESPACE
    ```
