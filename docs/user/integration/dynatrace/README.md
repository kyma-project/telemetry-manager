# Integrate With Dynatrace

## Overview

| Category| |
| - | - |
| Signal types | traces, metrics |
| Backend type | third-party remote |
| OTLP-native | yes, but dynatrace agent in parallel |

[Dynatrace](https://www.dynatrace.com) is an advanced Application Performance Management solution available as SaaS offering. It supports monitoring both the Kubernetes cluster itself and the workloads running on the cluster. To use all of its features, the proprietary agent technology of Dynatrace must be installed.

With the Kyma Telemetry module, you gain even more visibility by adding custom spans and Istio spans, as well as custom metrics. Get an introduction on how to set up Dynatrace and learn how to integrate the Kyma Telemetry module.

![setup](./../assets/dynatrace.drawio.svg)

## Table of Content

- [Integrate With Dynatrace](#integrate-with-dynatrace)
  - [Overview](#overview)
  - [Table of Content](#table-of-content)
  - [Prerequisistes](#prerequisistes)
  - [Prepare the Namespace](#prepare-the-namespace)
  - [Dynatrace Setup](#dynatrace-setup)
  - [Telemetry Module Setup](#telemetry-module-setup)
    - [Create Access Token](#create-access-token)
    - [Create Secret](#create-secret)
    - [Ingest Traces](#ingest-traces)
    - [Ingest Metrics (Experimental)](#ingest-metrics-experimental)

## Prerequisistes

- Kyma as the target deployment environment
- The [Telemetry module](https://kyma-project.io/#/telemetry-manager/user/README) is [enabled](https://kyma-project.io/#/02-get-started/08-install-uninstall-upgrade-kyma-module?id=install-uninstall-and-upgrade-kyma-with-a-module)
- Active Dynatrace environment with permissions to create new access tokens
- Helm 3.x if you want to deploy the [OpenTelemetry sample application](../opentelemetry-demo/README.md)

## Prepare the Namespace

1. Export your namespace you want to use for Dynatrace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export DYNATRACE_NS="dynatrace"
    ```

1. If you haven't created a namespace yet, do it now:

    ```bash
    kubectl create namespace $DYNATRACE_NS
    ```

## Dynatrace Setup

There are different ways to deploy Dynatrace on Kubernetes. All [deployment options](https://www.dynatrace.com/support/help/setup-and-configuration/setup-on-container-platforms/kubernetes/get-started-with-kubernetes-monitoring/deployment-options-k8s) are based on the [Dynatrace Operator](https://github.com/Dynatrace/dynatrace-operator).

1. Install Dynatrace with the namespace you prepared earlier.
   >**NOTE:** By default, Dynatrace uses the classic full-stack injection. However, for better stability, we recommend using the [cloud-native fullstack injection](https://docs.dynatrace.com/docs/setup-and-configuration/setup-on-k8s/installation/cloud-native-fullstack).

2. In the DynaKube resource, configure the correct `apiUrl` of your environemnt.

3. In the DynaKube resource, exclude Kyma system namespaces by adding the following snippet:

    ```yaml
    namespaceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
        operator: NotIn
        values:
        - kyma-system
        - istio-system
    ```

4. In the environment, go to **Settings > Cloud and virtualization > Kubernetes** and enable the relevant Kubernetes features, especially `Monitor annotated Prometheus exporters` to collect Istio metrics.

5. In the Dynatrace Hub, enable the **Istio Service Mesh** extension and annotate your services as outlined in the description.

6. If you have a workload exposing metrics in the Prometheus format, you can collect custom metrics in Prometheus format by [annotating the workload](https://docs.dynatrace.com/docs/platform-modules/infrastructure-monitoring/container-platform-monitoring/kubernetes-monitoring/monitor-prometheus-metrics). If the workload has an Istio sidecar, you must either weaken the mTLS setting for the metrics port by defining an [Istio PeerAuthentication](https://istio.io/latest/docs/reference/config/security/peer_authentication/#PeerAuthentication) or exclude the port from interception by the Istio proxy by placing an `traffic.sidecar.istio.io/excludeInboundPorts` annotaion on your Pod that lists the metrics port.

As a result, you see data arriving in your environment, advanced Kubernetes monitoring is possible, and Istio metrics are available.

## Telemetry Module Setup

Next, you set up the ingestion of custom and Istio span data. To collect custom metrics, you use the Dynatrace annotation approach, because the Telemetry module can easily collect OTLP-based metrics and push them centrally the the environment.

### Create Access Token

To push custom metrics and spans to Dynatrace, set up an [API Token](https://docs.dynatrace.com/docs/manage/access-control/access-tokens).

Follow the instructions in [Dynatrace: Generate an access token](https://docs.dynatrace.com/docs/manage/access-control/access-tokens#create-api-token) and select the following scopes:

- **Ingest metrics**
- **Ingest OpenTelemetry traces**

### Create Secret

To create a new Secret containing your access token, replace the `{API_TOKEN}` placeholder with the token you created and run the following command:

```bash
kubectl -n $DYNATRACE_NS create secret generic dynatrace-token --from-literal="apiToken=Api-Token {API_TOKEN}"
```

### Ingest Traces

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

1. Deploy the TracePipeline and replace the `{ENVIRONMENT_ID}` placeholder with the environment Id of your Dynatrace instance:

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

1. To find traces from your Kyma cluster in the Dynatrace UI, go to **Applications & Microservices** > **Distributed traces**.

### Ingest Metrics (Experimental)

1. Deploy the TracePipeline and replace the `{ENVIRONMENT_ID}` placeholder with the environment Id of Dynatrace SaaS:

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

1. To find metrics from your Kyma cluster in the Dynatrace UI, go to **Observe & Explore** > **Metrics**.
