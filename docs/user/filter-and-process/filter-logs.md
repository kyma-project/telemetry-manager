# Filter Logs

Filter logs from the OTLP, application, and Istio input to control which data your pipeline processes. You can define filters to include or exclude logs based on their source namespace, container, and other attributes.

## Overview

| Source      | Granularity                                       | If you omit the namespaces block... | To collect from **all** namespaces... | To collect from specific namespaces... |
| :---------- | :------------------------------------------------ | :---------------------------------- | :------------------------------------ | :------------------------------------- |
| OTLP (default) | Namespace                                         | **includes** system namespaces      | This is the default, no action needed. | Use the `include` or `exclude` selector |
| Application | Namespace, Container\*                            | **excludes** system namespaces      | Set the `system` attribute to `true`  | Use the `include` or `exclude` selector |
| Istio       | Namespace, Workload (`selector`), Log content (`filter.expression`) | n/a                                 | You apply the Istio `Telemetry` resource mesh-wide | You apply the Istio `Telemetry` resource to specific namespaces |

\* The `application` input provides an additional `containers` selector that behaves the same way as the `namespaces` selector.

## Filter OTLP Input by Namespaces

For logs and metrics, you can filter incoming OTLP data by namespaces. By default, all system namespaces are excluded.

The following example configures the pipeline to only accept OTLP data from `namespaceA` and `namespaceB`, and explicitly reject data from `namespaceC`:

```yaml
spec:
  input:
    otlp:
      namespaces:
        include:
          - namespaceA
          - namespaceB
        exclude:
          - namespaceC
```

To collect all namespaces without using any inclusion or exclusion list, use an empty struct syntax like `namespaces: {}`.

```yaml
spec:
  input:
    otlp:
      namespaces: {}
```

## Filter Application Logs by Namespace

You can control which namespaces to collect logs from using `include`, `exclude`, and `system` filters. The `include` and `exclude` filters are mutually exclusive.

- To collect logs from specific namespaces, use the `include` filter:

    ```yaml
      ...
      input:
        application:
          namespaces:
            include:
              - namespaceA
              - namespaceB
    ```

- To collect logs from all namespaces except specific ones, use the `exclude` filter:

    ```yaml
      ...
      input:
        application:
          namespaces:
            exclude:
              - namespaceC
    ```

## Filter Application Logs by Container

You can also filter logs based on the container name using `include` and `exclude` filters. These filters apply in addition to any namespace filters.

The following pipeline collects input from all namespaces excluding `kyma-system` and only from the `istio-proxy` containers:

```yaml
...
  input:
    application:
      enabled: true
      namespaces:
        exclude:
          - myNamespace
      containers:
        exclude:
          - myContainer
    otlp:
      ...
```

## Enable Istio Logs for a Namespace

1. Export the name of the target namespace as environment variable:

   ```bash
   export YOUR_NAMESPACE=<NAMESPACE_NAME>
   ```

2. Apply the Istio `Telemetry` resource:

   ```yaml
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

1. Export the name of the workload's namespace and label as environment variables:

    ```bash
    export YOUR_NAMESPACE=<NAMESPACE_NAME>
    export YOUR_LABEL=<LABEL>
    ```

2. Apply the Istio `Telemetry` resource with the selector:

   ```yaml
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

   ```yaml
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

## Filter Istio Logs

To reduce noise and cost, filter your Istio access logs. This is especially useful for filtering out traffic that doesn't use an HTTP-based protocol, as those log entries often lack useful details.

Add a `filter` expression to the same `accessLogging` block you used to enable the logs. The expression uses [Envoy attributes](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes) to define which log entries to keep.

The following example enables mesh-wide logging but only keeps logs that have a defined request protocol, effectively filtering out most non-HTTP traffic.

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