# Configure Application Logs

By default, a `LogPipeline` collects logs from all non-system namespaces that your containers write to `stdout` and `stderr`. You can configure this behavior in the **application** input section of your pipeline.

## Prerequisites

- You have enabled the Telemetry module.
- You have access to the Kyma dashboard or have [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed to use the command line.

## Context

You use the **spec.input.application** section in your `LogPipeline` Custom Resource to define how the log agent collects logs from your application Pods. You can specify which namespaces to include or exclude, enable collection from system namespaces, and control other collection behaviors. For a full list of parameters, see the [LogPipeline: Custom Resource Parameters](https://kyma-project.io/#/telemetry-manager/user/resources/02-logpipeline?id=custom-resource-parameters).

When you apply the `LogPipeline`, a log agent on each node starts collecting the log data, transforms it to the OTLP format, and sends it to your backend. For details, see [Transformation to OTLP Logs](../filter-and-process/transformation-to-otlp-logs.md).

## Enable or Disable Application Log Input

The **application** input is enabled by default. To create a pipeline that only accepts logs pushed via OTLP, you can disable the **application** input.

To enable collection of logs printed by containers to the `stdout/stderr` channel, define a LogPipeline that has the **application** section enabled as input:

```yaml
  ...
  input:
    application:
      enabled: false     # Default is true
```

By default, input is collected from all namespaces, except the system namespaces `kube-system`, `istio-system`, `kyma-system`, which are excluded by default.

## Collect Logs from System Namespaces

By default, logs from `kube-system`, `istio-system`, and `kyma-system` are excluded. To override this and collect logs from them, set the **system** attribute to true:

```yaml
  ...
  input:
    application:
      enabled: true
        namespaces:
          system: true
```

## Discard the Original Log Body

By default, the log agent preserves the original JSON log message by moving it to the **attributes."log.original"** field after parsing. For details, see [Transformation to OTLP Logs](../filter-and-process/transformation-to-otlp-logs.md).

To reduce data volume, you can disable this behavior. Set the parameter to `false` to discard the original JSON string after its contents are parsed into attributes.
