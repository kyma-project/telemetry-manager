# Filter Metrics

Filter metrics from the OTLP, Istio, Prometheus, and runtime input to control which data your pipeline processes. You can define filters to include or exclude metrics based on their source namespace and resource type.

## Overview

> [!TIP]
> The following settings filter data by source. For advanced, content-based filtering and transformation, use the OpenTelemetry Transformation Language (OTTL). For details, see [Transform and Filter with OTTL](./ottl-transform-and-filter/README.md).

| Source      | Granularity                    | Behavior without 'namespaces' Block | Collect from All Namespaces                       | Collect from Specific Namespaces |
| :---------- |:-------------------------------|:------------------------------------|:--------------------------------------------------| :------------------------------------- |
| OTLP (default) | Namespace                   | includes system namespaces          | This is the default, no action needed.            | Use the `include` or `exclude` filter |
| Istio       | Namespace                      | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter |
| Prometheus  | Namespace                      | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter |
| Runtime     | Namespace, Resource Type\*     | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter |

\* The **runtime** input provides additional filters for Kubernetes resources such as Pods or Nodes. For details, see [Select Resource Types](../collecting-metrics/runtime-input.md#select-resource-types).


## Filter Metrics by Namespace

For the all inputs (`otlp`, `prometheus`, `istio`, and `runtime`), you can filter incoming metrics by namespace. 

These filters only apply to metrics that have an associated namespace. The pipeline always collects any metrics that do not have a namespace.

The `include` and `exclude` filters are mutually exclusive.


- To collect metrics from specific namespaces, use the `include` filter:

  ```yaml
  ...
    input:
      <otlp | prometheus | istio | runtime>:
        enabled: true
        namespaces:
          include:
            - namespaceA
            - namespaceB
  ```

- To collect metrics from all namespaces **except** specific ones, use the `exclude` filter:

  ```yaml
  ...
    input:
      <otlp | prometheus | istio | runtime>:
        enabled: true
        namespaces:
          exclude:
            - namespaceA
            - namespaceB
  ```

## Collect Metrics From System Namespaces

For **otlp** metrics, system namespaces are included by default.

To include system namespaces for **prometheus**, **istio**, and **runtime** metrics without specifying any other namespaces, explicitly configure an empty namespace object: `namespaces: {}`:

```yaml
...
  input:
    <prometheus | istio | runtime>:
      enabled: true
      namespaces: {}
```
