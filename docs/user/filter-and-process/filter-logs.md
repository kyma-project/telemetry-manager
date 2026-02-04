# Filter Logs

Filter logs from the OTLP, application, and Istio input to control which data your pipeline processes. You can define filters to include or exclude logs based on their source namespace, container, and other attributes.

## Overview

> [!TIP]
> The following settings filter data by source. For advanced, content-based filtering and transformation, use the OpenTelemetry Transformation Language (OTTL). For details, see [Transform and Filter with OTTL](./ottl-transform-and-filter/README.md).

| Source         | Granularity                                                         | Behavior without 'namespaces' Block    | Collect from All Namespaces                       | Collect from Specific Namespaces                            |
|:---------------| :------------------------------------------------------------------ |:---------------------------------------|:--------------------------------------------------|:------------------------------------------------------------|
| OTLP (default) | Namespace                                                           | **includes** system namespaces         | This is the default, no action needed.                    |  Use the `include` or `exclude` filter                      |
| Application        | Namespace, Container\*                                              | excludes system namespaces             | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter                       |
| Istio          | Namespace, Workload (`selector`), Log content (`filter.expression`) | n/a                                    | Apply the Istio `Telemetry` resource mesh-wide    | Apply the Istio `Telemetry` resource to specific namespaces |

\* The **runtime** input provides an additional **containers** selector that behaves the same way as the **namespaces** selector.

## Filter OTLP Logs by Namespaces

You can filter incoming OTLP logs by namespace. By default, all system namespaces are included.

These filters only apply to logs that have an associated namespace. The pipeline always collects any logs that do not have a namespace.

The `include` and `exclude` filters are mutually exclusive.

- To collect logs from specific namespaces, use the `include` filter:

  ```yaml
  spec:
    input:
      otlp:
        namespaces:
          include:
            - namespaceA
            - namespaceB
  ```

- To collect OTLP logs from all namespaces **except** specific ones, use the `exclude` filter:

  ```yaml
  spec:
    input:
      otlp:
        namespaces:
          exclude:
            - namespaceA
            - namespaceB
  ```

## Filter Application Logs by Namespace

You can filter incoming application logs by namespace. The `include` and `exclude` filters are mutually exclusive.


- To collect logs from specific namespaces, use the `include` filter:

    ```yaml
      ...
      input:
        runtime:
          namespaces:
            include:
              - namespaceA
              - namespaceB
    ```

- To collect logs from all namespaces except specific ones, use the `exclude` filter:

    ```yaml
      ...
      input:
        runtime:
          namespaces:
            exclude:
              - namespaceC
    ```

## Collect Application Logs from System Namespaces

By default, application logs from `kube-system`, `istio-system`, and
`kyma-system` are excluded. To override this and collect logs from them, set the **namespaces** attribute to an empty object:

```yaml
  ...
  input:
    runtime:
      enabled: true
      namespaces: {}
```

## Filter Application Logs by Container

You can also filter logs based on the container name with `include` and
`exclude` filters. These filters apply in addition to any namespace filters.

The following pipeline collects input from all namespaces excluding `kyma-system` and only from the
`istio-proxy` containers:

```yaml
...
input:
  runtime:
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

## Select Istio Logs from a Specific Application

To limit logging to a single application within a namespace, configure label-based selection for this workload with a [selector](https://istio.io/latest/docs/reference/config/type/workload-selector/#WorkloadSelector) in the Istio
`Telemetry` resource.

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

## Filter Istio Logs by Content

To reduce noise and cost, filter your Istio access logs. This is especially useful for filtering out traffic that doesn't use an HTTP-based protocol, as those log entries often lack useful details.

Add a `filter` expression to the same
`accessLogging` block that you used to enable the logs. The expression uses [Envoy attributes](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes) to define which log entries to keep.

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
