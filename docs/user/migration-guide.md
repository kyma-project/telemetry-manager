# Migrate Your Pipelines from v1alpha1 to v1beta1

Telemetry module v1beta1 introduces a number of changes to the Telemetry pipeline API, including some breaking changes.

This guide outlines the key changes and steps required to migrate your Telemetry pipelines from version v1alpha1 to v1beta1. The new version introduces several renamed fields and modifications to improve API functionality and usability.



## Key Changes

The diagram below summarizes the main changes between v1alpha1 and v1beta1 of the Telemetry pipeline API.

![LogPipeline Migration Changes](./assets/logpipeline-migration.png)
![MetricPipeline Migration Changes](./assets/metricpipeline-migration.png)

The following sections provide details on the most important changes in the pipeline API.

### Renamed Fields

The following fields are renamed in v1beta1 compared to v1alpha1. Most fields are renamed to align with common conventions or to improve clarity, while some fields have their logic inverted (e.g., changing from `disabled` to `enabled`).

#### LogPipeline

| v1alpha1 Field                                   | v1beta1 Field                             | Description                                                                                           |
|--------------------------------------------------|-------------------------------------------|-------------------------------------------------------------------------------------------------------|
| `spec.input.otlp.disabled`                       | `spec.input.otlp.enabled`                 | Changed from `disabled` to `enabled`, with the logic inverted                                         |
| `spec.input.application`                         | `spec.input.runtime`                      | Changed from `application` to `runtime` to align with other pipelines                                 | 
| `spec.output.http.tls.disabled`                  | `spec.output.http.tls.insecure`           | Changed from `disabled` to `insecure` to align with OTLP TLS configuration                            |
| `spec.output.http.tls.skipCertificateValidation` | `spec.output.http.tls.insecureSkipVerify` | Changed from `skipCertificateValidation` to `insecureSkipVerify` to align with OTLP TLS configuration |

#### MetricPipeline

| v1alpha1 Field                                   | v1beta1 Field                             | Description                                                                                         |
|--------------------------------------------------|-------------------------------------------|-----------------------------------------------------------------------------------------------------|
| `spec.input.otlp.disabled`                       | `spec.input.otlp.enabled`                 | Changed from `disabled` to `enabled`, with the logic inverted                                       |

### System Namespace Exclusion in LogPipeline

In v1alpha1 LogPipeline, the `spec.application.namespaces.system` field allowed users to include or exclude system namespaces from log collection by providing a boolean value. With v1beta1, this field has been removed to simplify namespace exclusion. Instead, if neither an exclusion list nor an inclusion list is provided, system namespaces are excluded by default. To include system namespaces, you must now provide an empty object to the exclusion list.

The following examples illustrate the new behavior:

```yaml
spec:
  input:
    runtime:
      enabled: true
      namespaces:
        exclude: {}  # This will include system namespaces
```

```yaml
spec:
  input:
    runtime: # if neither exclude nor include is provided, system namespaces are excluded by default
      enabled: true
```
