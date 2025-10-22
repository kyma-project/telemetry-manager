# Configure Istio Access Logs

To monitor traffic in your service mesh, configure Istio to send access logs. The LogPipeline automatically receives these logs through its default OTLP input.

## Prerequisites

- You have the Istio module in your cluster. See [Quick Install](https://kyma-project.io/#/02-get-started/01-quick-install).
- You have access to Kyma dashboard. Alternatively, if you prefer CLI, you need [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

## Context

Istio access logs help you monitor the "four golden signals" (latency, traffic, errors, and saturation) and troubleshoot anomalies.

By default, these logs are disabled because they can generate a high volume of data. To collect them, you apply an [Istio](https://istio.io/latest/docs/reference/config/telemetry/) `Telemetry` resource to a specific namespace, for a specific workload, or for the entire mesh.

After enabling Istio access logs, reduce data volume and costs by filtering them (see [Filter Logs](../filter-and-process/filter-logs.md)).

> **Caution:**
> Enabling access logs, especially for the entire mesh, can significantly increase log volume and may lead to higher storage costs. Enable this feature only for the resources or components that you want to monitor.

The Istio module provides a preconfigured [extension provider](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig-ExtensionProvider) called `kyma-logs`, which tells Istio to send access logs to the Telemetry module's OTLP endpoint. If your LogPipeline uses the legacy **http** output, you must use the `stdout-json` provider instead.

## Enable Istio Logs for a Namespace

1. Export the name of the target namespace as environment variable:

   ```bash
   export YOUR_NAMESPACE={NAMESPACE_NAME}
   ```

2. Apply the Istio `Telemetry` resource:

    ```yaml
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: access-config
      namespace: $YOUR_NAMESPACE
    spec:
      accessLogging:
        - providers:
          - name: kyma-logs
    ```

3. Verify that the resource is applied to the target namespace:

   ```bash
   kubectl -n $YOUR_NAMESPACE get telemetries.telemetry.istio.io
   ```

## Enable Istio Logs for a Specific Workload

To configure label-based selection of workloads, use a [selector](https://istio.io/latest/docs/reference/config/type/workload-selector/#WorkloadSelector).

1. Export the name of the workload's namespace and label as environment variables:

    ```bash
    export YOUR_NAMESPACE={NAMESPACE_NAME}
    export YOUR_LABEL={LABEL}
    ```

2. Apply the Istio `Telemetry` resource with the selector:

    ```yaml
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: access-config
      namespace: $YOUR_NAMESPACE
    spec:
      selector:
        matchLabels:
          service.istio.io/canonical-name: $YOUR_LABEL
      accessLogging:
        - providers:
          - name: kyma-logs
    ```

3. Verify that the resource is applied to the target namespace:

    ```bash
    kubectl -n $YOUR_NAMESPACE get telemetries.telemetry.istio.io
    ```

## Enable Istio Logs for the Ingress Gateway

To monitor all traffic entering your mesh, enable access logs on the Istio Ingress Gateway (instead of the individual proxies of your workloads).

1. Apply the Istio `Telemetry` resource to the `istio-system` namespace, selecting the gateway Pods:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: access-config
      namespace: istio-system
    spec:
      selector:
        matchLabels:
          istio: ingressgateway
      accessLogging:
        - providers:
          - name: kyma-logs
    EOF
    ```

2. Verify that the resource is applied to the `istio-system` namespace:

    ```bash
    kubectl -n istio-system get telemetries.telemetry.istio.io
    ```

## Enable Istio Logs for the Entire Mesh

You can enable access logs globally for all proxies in the mesh. Use this option with caution due to the high data volume.

> [!NOTE]
> You can only have one mesh-wide Istio `Telemetry` resource. If you also plan to enable Istio tracing (see [Configure Istio Tracing](./../collecting-traces/istio-support.md)), configure both access logging and tracing in this single resource.

1. Apply the Istio `Telemetry` resource to the `istio-system` namespace:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: mesh-default
      namespace: istio-system
    spec:
      accessLogging:
        - providers:
          - name: kyma-logs
    EOF
    ```

2. Verify that the resource is applied to the `istio-system` namespace:

    ```bash
    kubectl -n istio-system get telemetries.telemetry.istio.io
    ```
