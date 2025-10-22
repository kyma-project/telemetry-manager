# Filter Metrics

Filter metrics from the OTLP, Istio, Prometheus, and runtime input to control which data your pipeline processes. You can define filters to include or exclude metrics based on their source namespace and resource type.

## Overview

| Source      | Granularity                               | If you omit the namespaces block... | To collect from **all** namespaces... | To collect from specific namespaces... |
| :---------- | :---------------------------------------- | :---------------------------------- | :------------------------------------ | :------------------------------------- |
| OTLP (default) | Namespace                                 | includes system namespaces          | This is the default, no action needed. | Use the `include` or `exclude` filter |
| Istio       | Namespace                                 | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter |
| Prometheus  | Namespace                                 | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter |
| Runtime     | Namespace, Resource Type\*                | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` filter |

\* The **runtime** input provides additional filters for Kubernetes resources such as Pods or Nodes. For details, see [Select Resource Types](../collecting-metrics/runtime-input.md#select-resource-types).


## Filter Metrics by Namespace

For the all inputs (`otlp`, `prometheus`, `istio`, and `runtime`), you can filter incoming metrics by namespace. The `include` and `exclude` filters are mutually exclusive.

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
    <prometheus | runtime>:
      enabled: true
      namespaces: {}
```