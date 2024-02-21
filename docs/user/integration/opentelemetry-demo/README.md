# Integrate OpenTelemetry Demo App

## Overview

| Category| |
| - | - |
| Signal types | traces, metrics |
| Backend type | custom in-cluster, third-party remote |
| OTLP-native | yes |

Learn how to install the OpenTelemetry [demo application](https://github.com/open-telemetry/opentelemetry-demo) in a Kyma cluster using a provided [Helm chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-demo). The demo application will be configured to push trace data using OTLP to the collector that's provided by Kyma, so that they are collected together with the related Istio trace data.

![setup](./../assets/otel-demo.drawio.svg)

## Table of Content

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Advanced](#advanced)
- [Clean Up](#clean-up)

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](../../README.md) is [enabled](https://kyma-project.io/#/02-get-started/01-quick-install)
- [Kubectl version that is within one minor version (older or newer) of `kube-apiserver`](https://kubernetes.io/releases/version-skew-policy/#kubectl)
- Helm 3.x

## Installation

### Preparation

1. Export your Namespace as a variable. Replace the `{namespace}` placeholder in the following command and run it:

    ```bash
    export K8S_NAMESPACE="{namespace}"
    ```

1. If you haven't created a Namespace yet, do it now:

    ```bash
    kubectl create namespace $K8S_NAMESPACE
    ```

1. To enable Istio injection in your Namespace, set the following label:

    ```bash
    kubectl label namespace $K8S_NAMESPACE istio-injection=enabled
    ```

1. Export the Helm release name that you want to use. The release name must be unique for the chosen Namespace. Be aware that all resources in the cluster will be prefixed with that name. Run the following command:

    ```bash
    export HELM_OTEL_RELEASE="otel"
    ```

1. Update your Helm installation with the required Helm repository:

    ```bash
    helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
    helm repo update
    ```

### Activate a Kyma TracePipeline

1. Provide a tracing backend and activate it.
   Install [Jaeger in-cluster](../jaeger/README.md) or provide a custom backend supporting the OTLP protocol.
2. Activate the Istio tracing feature.
To [enable Istio](../../03-traces.md#step-2-enable-istio-tracing) to report span data, apply an Istio telemetry resource and set the sampling rate to 100%. This approach is not recommended for production.

   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: telemetry.istio.io/v1alpha1
   kind: Telemetry
   metadata:
     name: tracing-default
     namespace: istio-system
   spec:
     tracing:
     - providers:
       - name: "kyma-traces"
       randomSamplingPercentage: 100.00
   EOF
   ```

### Install the Application

Run the Helm upgrade command, which installs the chart if not present yet.

```bash
helm upgrade --version 0.26.1 --install --create-namespace -n $K8S_NAMESPACE $HELM_OTEL_RELEASE open-telemetry/opentelemetry-demo -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/opentelemetry-demo/values.yaml
```

The previous command uses the [values.yaml](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/opentelemetry-demo/values.yaml) provided in this `opentelemetry-demo` folder, which contains customized settings deviating from the default settings. The customizations in the provided `values.yaml` cover the following areas:

- Disable the observability tooling provided with the chart
- Configure Kyma Telemetry instead
- Extend memory limits of the demo apps to avoid crashes caused by memory exhaustion
- Adjust initContainers and services of demo apps to work proper with Istio

Alternatively, you can create your own `values.yaml` file and adjust the command.

### Verify the Application

To verify that the application is running properly, set up port forwarding and call the respective local hosts.

1. Verify the frontend:

   ```bash
   kubectl -n $K8S_NAMESPACE port-forward svc/$HELM_OTEL_RELEASE-frontend 8080
   ```

   ```bash
   open http://localhost:8080
   ````

2. Verify that traces arrive in the Jaeger backend. If you deployed [Jaeger in your cluster](../jaeger/README.md) using the same Namespace as used for the demo app, run:

   ```bash
   kubectl -n $K8S_NAMESPACE port-forward svc/tracing-jaeger-query 16686
   ```

   ```bash
   open http://localhost:16686
   ````

3. Enable failures with the feature flag service:

   ```bash
   kubectl -n $K8S_NAMESPACE port-forward svc/$HELM_OTEL_RELEASE-featureflagservice 8081
   ```

   ```bash
   open http://localhost:8081
   ````

4. Generate load with the load generator:

   ```bash
   kubectl -n $K8S_NAMESPACE port-forward svc/$HELM_OTEL_RELEASE-loadgenerator 8089
   ```

   ```bash
   open http://localhost:8089
   ```

## Advanced

### Browser Instrumentation and Missing Root Spans

The frontend application of the demo uses [browser instrumentation](https://opentelemetry.io/docs/demo/services/frontend/#browser-instrumentation). Because of that, the root span of a trace is created externally to the cluster and is not captured with the described setup. In Jaeger, you can see a warning at the first span indicating that there is a parent span that is not being captured.

To capture the spans reported by the browser, you must expose the trace endpoint of the collector and configure the frontend to report the spans to that exposed endpoint.

1. To expose the frontend web app and the relevant trace endpoint, define an Istio Virtualservice as in the following example:

    >**CAUTION** The example shows an insecure way of exposing the trace endpoint. Do not use it in production!

    ```bash
    export CLUSTER_DOMAIN={my-domain}
    
    cat <<EOF | kubectl -n $K8S_NAMESPACE apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: frontend
    spec:
      gateways:
      - kyma-system/kyma-gateway
      hosts:
      - frontend.$CLUSTER_DOMAIN
      http:
      - route:
        - destination:
            host: telemetry-otlp-traces.kyma-system.svc.cluster.local
            port:
              number: 4318
        match:
        - uri:
            prefix: "/v1/traces"
        corsPolicy:
          allowOrigins:
          - prefix: https://frontend.$CLUSTER_DOMAIN
          allowHeaders:
          - Content-Type
          allowMethods:
          - POST
          allowCredentials: true
      - route:
        - destination:
            host: $HELM_OTEL_RELEASE-frontend.$K8S_NAMESPACE.svc.cluster.local
            port:
              number: 8080
    EOF
    ```

2. Update the frontend configuration. To do that, run the following Helm command and set the additional configuration:

    ```bash
    helm upgrade --version 0.22.2 --install --create-namespace -n $K8S_NAMESPACE $HELM_OTEL_RELEASE open-telemetry/opentelemetry-demo \
    -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/opentelemetry-demo/values.yaml \
    --set 'components.frontend.envOverrides[1].name=PUBLIC_OTEL_EXPORTER_OTLP_TRACES_ENDPOINT' \
    --set "components.frontend.envOverrides[1].value=https://frontend.$CLUSTER_DOMAIN/v1/traces"
    ```

    As a result, the web application is accessible in your browser at `https://frontend.$CLUSTER_DOMAIN`.

3. To check whether requests to the traces endpoint are successful, use the developer tools of the browser. Now, every trace should have a proper root span recorded.

## Clean Up

When you're done, you can remove the example and all its resources from the cluster by calling Helm:

```bash
helm delete -n $K8S_NAMESPACE $HELM_OTEL_RELEASE
```
