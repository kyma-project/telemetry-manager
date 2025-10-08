# Configure Istio Access Logs

Enable Istio access logs to get details about traffic to your workloads in the Istio service mesh. You can use these logs to monitor the "four golden signals" (latency, traffic, errors, and saturation) and to troubleshoot anomalies.

## Prerequisites

- You have the Istio module enabled in your cluster. See [Adding and Deleting a Kyma Module](ADD LINK).
- You have access to Kyma dashboard. Alternatively, if you prefer CLI, you need [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

## Context

[Istio access logs](https://istio.io/latest/docs/tasks/observability/logs/access-log/) are disabled by default because they can generate a high volume of data. To collect them, you apply an Istio `Telemetry` resource to a specific namespace, a specific workload, or the entire mesh. After you enable the logs, you can add a filter to reduce noise and focus on relevant data.

> **Caution:**
> Enabling access logs, especially for the entire mesh, can significantly increase log volume and may lead to higher storage costs. Enable this feature only for the specific resources you need to monitor.

The Istio module provides a preconfigured [extension provider](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig-ExtensionProvider) called `kyma-logs`. This provider tells Istio to send access logs to the Telemetry module's OTLP endpoint. If your `LogPipeline` uses the legacy **http** output, you must use the `stdout-json` provider instead.

## Enable Istio Logs for a Namespace

1. Export the name of the target namespace as environment variable:

   ```bash
   export YOUR_NAMESPACE={NAMESPACE_NAME}
   ```

2. Apply the Istio `Telemetry` resource:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: access-config
      namespace: $YOUR_NAMESPACE
    spec:
      accessLogging:
        - providers:
          - name: kyma-logs
    EOF
    ```

3. Verify that the resource is applied to the target namespace:

   ```bash
   kubectl -n $YOUR_NAMESPACE get telemetries.telemetry.istio.io
   ```

## Enable Istio Logs for a Specific Workload

To configure label-based selection of workloads, use a [selector](https://istio.io/latest/docs/reference/config/type/workload-selector/#WorkloadSelector).

1. Export the name of the workloads' namespace and their label as environment variables:

    ```bash
    export YOUR_NAMESPACE={NAMESPACE_NAME}
    export YOUR_LABEL={LABEL}
    ```

2. Apply the Istio `Telemetry` resource with the selector:

    ```bash
    cat <<EOF | kubectl apply -f -
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
    EOF
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
> You can only have one mesh-wide Istio `Telemetry` resource. If you also plan to enable Istio tracing (see [Configure Istio Tracing]()), configure both access logging and tracing in this single resource.


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
