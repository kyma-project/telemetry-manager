# Filter Metrics

Filter metrics from the OTLP, Istio, Prometheus, and runtime input to control which data your pipeline processes. You can define filters to include or exclude metrics based on their source namespace and resource type.

## Overview

| Source      | Granularity                               | If you omit the namespaces block... | To collect from **all** namespaces... | To collect from specific namespaces... |
| :---------- | :---------------------------------------- | :---------------------------------- | :------------------------------------ | :------------------------------------- |
| OTLP (default) | Namespace                                 | includes system namespaces          | This is the default, no action needed. | Use the `include` or `exclude` selector |
| Istio       | Namespace                                 | includes system namespaces          | This is the default, no action needed. | Use the `include` or `exclude` selector |
| Prometheus  | Namespace                                 | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` selector |
| Runtime     | Namespace, Resource Type\*                | excludes system namespaces          | Add `namespaces: {}` to the input's configuration | Use the `include` or `exclude` selector |

\* The `runtime` input provides additional filters for Kubernetes resources such as Pods or Nodes. For details, see [Select Resource Types](../metrics/runtime-input.md#select-resource-types).

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
...
```

To collect all namespaces without using any inclusion or exclusion list, use an empty struct syntax like `namespaces: {}`.

```yaml
spec:
  input:
    otlp:
      namespaces: {}
...
```

## Collect Istio Metrics From System Namespaces

For the `istio` input, system namespaces are included by default.

To include system namespaces for `prometheus` and `runtime` metrics without specifying any other namespaces, explicitly configure an empty namespace object: `namespaces: {}`.

```yaml
...
  input:
    <prometheus | runtime>:
      enabled: true
      namespaces: {}
```

## Collect Runtime Metrics From Specific Namespaces

The following example collects runtime metrics **only** from the `foo` and `bar` namespaces:

```yaml
...
  input:
    runtime:
      enabled: true
      namespaces:
        include:
          - foo
          - bar
```

## Drop Runtime Metrics From Specific Namespaces

The following example collects runtime metrics from all namespaces **except** the `foo` and `bar` namespaces:

```yaml
...
  input:
    runtime:
      enabled: true
      namespaces:
        exclude:
          - foo
          - bar
```