# Logs Istio Support

Use the Istio Telemetry API to selectively enable the Istio access logs and filter them if needed.

## Prerequisites

* You have the Istio module added.

## Context

You can enable [Istio access logs](https://istio.io/latest/docs/tasks/observability/logs/access-log/) to provide fine-grained details about the access to workloads that are part of the Istio service mesh. This can help indicate the four “golden signals” of monitoring (latency, traffic, errors, and saturation) and troubleshooting anomalies.
The Istio setup shipped with the Kyma Istio module provides a pre-configured [extension provider](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig-ExtensionProvider) called `kyma-logs` for access logs based on OTLP. It can be enabled via the [Istio Telemetry API](https://istio.io/latest/docs/tasks/observability/telemetry), which configures the Istio proxies to push access logs to the push-endpoint of the telemetry module.

> [!WARNING]
> When using LogPipelines with `http` output, then the integration via OTLP is not supported and the legacy extension provider `stdout-json` needs to be used
> Enabling access logs may drastically increase logs volume and might quickly fill up your log storage.

For more details on how to enable the Istio tracing using the same API, see [Traces Istio Support](./../traces/README.md#istio) 

## Configuration

Use the Telemetry API to selectively enable Istio access logs. See:

<!-- no toc -->
* [Configure Istio Access Logs for a Namespace](#configure-istio-access-logs-for-a-namespace)
* [Configure Istio Access Logs for a Selective Workload](#configure-istio-access-logs-for-a-selective-workload)
* [Configure Istio Access Logs for a Specific Gateway](#configure-istio-access-logs-for-a-selective-gateway)
* [Configure Istio Access Logs for the Entire Mesh](#configure-istio-access-logs-for-the-entire-mesh)
* [Filter Access Logs](#filter-access-logs)

To filter the enabled access logs, you can edit the Telemetry API by adding a filter expression. See [Filter Access logs](#filter-access-logs).

### Configure Istio Access Logs for a Namespace

1. Export the name of the namespace for which you want to configure Istio access logs.

    ```bash
    export YOUR_NAMESPACE={NAMESPACE_NAME}
    ```

2. To apply the configuration, run:

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

3. To verify that the resource is applied, run:

    ```bash
    kubectl -n $YOUR_NAMESPACE get telemetries.telemetry.istio.io
    ```

### Configure Istio Access Logs for a Selective Workload

To configure label-based selection of workloads, use a [selector](https://istio.io/latest/docs/reference/config/type/workload-selector/#WorkloadSelector).

1. Export the name of the workloads' namespace and their label as environment variables:

    ```bash
    export YOUR_NAMESPACE={NAMESPACE_NAME}
    export YOUR_LABEL={LABEL}
    ```

2. To apply the configuration, run:

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

3. To verify that the resource is applied, run:

    ```bash
    kubectl -n $YOUR_NAMESPACE get telemetries.telemetry.istio.io
    ```

### Configure Istio Access Logs for a Selective Gateway

Instead of enabling the access logs for all the individual proxies of the workloads you have, you can enable the logs for the proxy used by the related Istio Ingress Gateway.

1. To apply the configuration, run:

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

2. To verify that the resource is applied, run:

    ```bash
    kubectl -n istio-system get telemetries.telemetry.istio.io
    ```

### Configure Istio Access Logs for the Entire Mesh

Enable access logs for all individual proxies of the workloads and Istio Ingress Gateways.

1. To apply the configuration, run:

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

2. To verify that the resource is applied, run:

    ```bash
    kubectl -n istio-system get telemetries.telemetry.istio.io
    ```

> [!NOTE]
> There can be only one Istio Telemetry resource on global mesh level. If you also enable Istio tracing, assure that the configuration happens in the same resource. See [Traces Istio Support](./../traces/istio-support.md)

### Filter Access Logs

Often, access logs emitted by Envoy do not contain data relevant to your observations, especially when the traffic is not based on an HTTP-based protocol. In such a situation, you can directly configure the Istio Envoys to filter out logs using a filter expression. To filter access logs, you can leverage the same [Istio Telemetry API](https://istio.io/latest/docs/reference/config/telemetry/#AccessLogging) that you used to enable them. To formulate which logs to **keep**, define a filter expression leveraging the typical [Envoy attributes](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes).

For example, to filter out all logs having no protocol defined (which is the case if they are not HTTP-based), you can use a configuration similar to this example:

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
 name: mesh-default
 namespace: istio-system
spec:
 accessLogging:
 - filter:
     expression: 'has(request.protocol)'
   providers:
   - name: kyma-logs
```
