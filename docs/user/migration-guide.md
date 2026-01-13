# Migrate Your Pipelines from v1alpha1 to v1beta1

Migrate your Telemetry pipeline resources to the stable v1beta1 API version. The v1beta1 API is more consistent, clear, and aligned with industry standards.

This migration involves breaking changes. You must update the **apiVersion**, rename several fields, and, for LogPipeline resources, adjust how you configure namespace selection.



## Context

The migration from v1alpha1 and v1beta1 mostly affects LogPipeline and MetricPipeline resources. For TracePipeline resources, you only have to change the **apiVersion**.

![LogPipeline Migration Changes](./assets/logpipeline-migration.png)
![MetricPipeline Migration Changes](./assets/metricpipeline-migration.png)

In your Telemetry pipeline resources, see if one of the following breaking changes between v1alpha1 and v1beta1 affects you:

| Pipeline                    | v1alpha1 Field                                 | v1beta1 Field                           | Migration Action                                                     |
|-----------------------------|------------------------------------------------|-----------------------------------------|----------------------------------------------------------------------|
| LogPipeline                 | spec.input.application                         | spec.input.runtime                      | Rename the field.                                                    | LogPipeline                 | spec.output.http.tls.disabled                  | spec.output.http.tls.insecure           | Rename the field.                                                    |
| LogPipeline                 | spec.output.http.tls.skipCertificateValidation | spec.output.http.tls.insecureSkipVerify | Rename the field.                                                    |
| LogPipeline, MetricPipeline | spec.input.otlp.disabled                       | spec.input.otlp.enabled                 | Rename the field and invert the boolean value (e.g., false -> true). |
| LogPipeline                 | spec.input.application.namespaces.system       | (Removed)                               | To include system namespaces, use spec.input.runtime.namespaces: {}. |


## Prerequisites

- You have an active Kyma cluster with the Telemetry module added.
- You have one or more Telemetry pipeline resources that use the `telemetry.kyma-project.io/v1alpha1` API.

## Procedure

1. In each of your LogPipeline, MetricPipeline, and TracePipeline YAML files, change the **apiVersion** to `telemetry.kyma-project.io/v1beta1`.

2. For LogPipeline and MetricPipeline resources, find and replace the fields that were renamed in v1beta1.
   1. Update the OTLP input field:
      - **spec.input.otlp.disabled** becomes **spec.input.otlp.enabled**.
      - You must also invert the boolean value (for example, `disabled: false` becomes `enabled: true`).

   2. For LogPipeline resources, also update the following fields:
      - **spec.input.application** becomes **spec.input.runtime**.
      - In the http output, **spec.output.http.tls.disabled** becomes **spec.output.http.tls.insecure**.
      - In the http output, **spec.output.http.tls.skipCertificateValidation** becomes **spec.output.http.tls.insecureSkipVerify**.

3.  For LogPipeline resources, if you want to include system namespaces for application logs, update the system namespace selection.
   By default, system namespaces are excluded (as in v1alpha1), but v1beta1 removes the **spec.input.application.namespaces.system** field. To include application logs from system namespaces (like `kyma-system`), you must now provide an empty object `({})` for the **namespaces** selector. For details, see [Filter Application Logs by Namespace](https://kyma-project.io/external-content/telemetry-manager/docs/user/filter-and-process/filter-logs.html#filter-application-logs-by-namespace).
   ```yaml
   spec:
    input:
      runtime:
        enabled: true
        namespaces:
          exclude: []  # This includes system namespaces
   ```

4. Validate and apply your updated configuration with kubectl.

## Result
Your pipelines are now updated to the v1beta1 API. The Telemetry module begins using the new configuration.

To confirm the migration was successful, check the status conditions of your new pipelines. A healthy pipeline shows `True` for all conditions.
