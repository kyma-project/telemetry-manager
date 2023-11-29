# Integrate with Dynatrace

## Overview

The Kyma Telemetry module supports you in integrating with observability backends in a convenient way. The following example outlines how to integrate with [Dynatrace](https://www.dynatrace.com) as a backend. With Dynatrace, you can get all your monitoring and tracing data into one observability backend to achieve real Application Performance Management with monitoring data in context. Apart from [installing Dynatrace Operator](https://github.com/Dynatrace/dynatrace-operator) and leveraging all of the benefits of Dynatrace, you can also integrate custom metrics and traces with Dynatrace.
This tutorial covers only trace and metric ingestion. Log ingestion is not covered.

![overview](./assets/integration.drawio.svg)

## Prerequisistes

- Kyma as the target deployment environment
- Dynatrace environment with permissions to create new access tokens
- Helm 3.x if you want to deploy open-telemetry sample application
- Deploy [Dynatrace Operator](https://www.dynatrace.com/support/help/setup-and-configuration/setup-on-k8s/quickstart) (recommended)

## Installation

### Preparation

1. Export your Namespace you want to use for Dynatrace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export DYNATRACE_NS="{NAMESPACE}"
    ```

1. If you haven't created a Namespace yet, do it now:

    ```bash
    kubectl create namespace $DYNATRACE_NS
    ```

### Create access token

Before connecting Kyma and Dynatrace, create a Dynatrace access token:

1. In the Dynatrace navigation, go to the **Manage** > **Access tokens** > **Generate new token**.
1. Type the name you want to give to this token. If needed, set an expiration date.
1. Select the following scopes:
   - **Ingest metrics**
   - **Ingest OpenTelemetry traces**
1. Click **Generate token**.
1. Copy and save the generated token.

### Create Secret

1. To create a new secret containing your access token. In the following command, replace the `{API_TOKEN}` placeholder with the previously created token and run it::

    ```bash
    kubectl -n $DYNATRACE_NS create secret generic dynatrace-token --from-literal="apiToken=Api-Token {API_TOKEN}"
    ```

### Ingest Traces

#### Enable Collecting Istio Traces

By default, the tracing feature of the Istio service mesh is disabled to save resources for the case that no TracePipeline is defined.  To enable emitting traces with the kyma-traces Istio extension provider, create a Telemetry custom resource in the `istio-system` Namespace:

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
    randomSamplingPercentage: 1.0
EOF
```

This configuration samples 1% of all requests. if you want to change the sampling rate, adjust the **randomSamplingPercentage** property.

#### Create Kyma TracePipeline

1. Replace the `{ENVIRONMENT_ID}` placeholder with the environment Id of your Dynatrace instance:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: TracePipeline
    metadata:
        name: dynatrace
    spec:
        output:
            otlp:
                endpoint:
                    value: https://{ENVIRONMENT_ID}.live.dynatrace.com/api/v2/otlp
                headers:
                    - name: Authorization
                      valueFrom:
                          secretKeyRef:
                              name: dynatrace-token
                              namespace: ${DYNATRACE_NS}
                              key: apiToken
                protocol: http
    EOF
    ```

    If you manage Dynatrace yourself, the link to the Dynatrace API endpoint is `https://{YOUR_DOMAIN}/e/{ENVIRONMENT_ID}/api/v2/otlp`
1. To find traces from your Kyma cluster in the Dynatrace UI, go to **Applications & Microservices** > **Distributed traces**.

### Ingest Metrics

> **NOTE:** The `MetricPipeline` feature is still under development. To understand the current progress, watch this [epic](https://github.com/kyma-project/kyma/issues/13079).

#### Create Kyma MetricPipeline

1. Replace the `{ENVIRONMENT_ID}` placeholder with the environment Id of Dynatrace SaaS:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: MetricPipeline
    metadata:
        name: dynatrace
    spec:
        output:
            otlp:
                endpoint:
                    value: https://{ENVIRONMENT_ID}.live.dynatrace.com/api/v2/otlp
                headers:
                    - name: Authorization
                      valueFrom:
                          secretKeyRef:
                              name: dynatrace-token
                              namespace: ${DYNATRACE_NS}
                              key: apiToken
                protocol: http
    EOF
    ```

    If you manage Dynatrace yourself, the link to the Dynatrace API endpoint is `https://{YOUR_DOMAIN}/e/{ENVIRONMENT_ID}/api/v2/otlp`

## Verify the results by deploying sample apps

To verify metrics and traces arrival, we adapted the [OpenTelemetry Demo Application](../opentelemetry-demo/README.md).

### Preparation

1. Export your Namespace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export KYMA_NS="{NAMESPACE}"
    ```

1. If you haven't created a Namespace yet, do it now:

    ```bash
    kubectl create namespace $KYMA_NS
    ```

1. Update your Helm installation with the required Helm repository:

    ```bash
    helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
    helm repo update
    ```

### Install the Helm chart

1. Run the Helm upgrade command, which installs the chart if not present yet.

   ```bash
   helm upgrade --version 0.22.2 --install --create-namespace -n $KYMA_NS otel open-telemetry/opentelemetry-demo -f ./sample-app/values.yaml

2. To access your application, port-forward it to the frontend:

   ```bash
   kubectl -n $KYMA_NS port-forward svc/otel-frontendproxy 8080

### Access metrics and traces

1. To access metrics, open your **Dynatrace Manager** and go to **Observe and explore** > **Metrics**.
1. To access traces, open your **Dynatrace Manager** and go to **Applications & Microservices** > **Distributed traces**.
