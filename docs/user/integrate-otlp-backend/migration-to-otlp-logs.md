# Migrate Your LogPipeline From HTTP to OTLP

To use the OpenTelemetry Protocol (OTLP) for sending logs, you must migrate your LogPipeline from the `http` or `custom` output to the `otlp` output. With OTLP, you can correlate logs with traces and metrics, collect logs pushed directly from applications, and use features available only for the OTLP-based stack.

## Prerequisites

* You have an active Kyma cluster with the Telemetry module added.
* You have one or more LogPipeline resources that use the `http` or `custom` output.
* Your observability backend has an OTLP ingestion endpoint.
  If your backend doesn't support OTLP natively, you must run a custom OTel Collector as gateway between the Telemetry module and the target backend.

## Context

When you want to migrate to the `otlp` output, create a new LogPipeline. To prevent data loss, run it in parallel with your existing pipeline. After verifying that the new pipeline works correctly, you can delete the old one.

You can't modify an existing LogPipeline to change its output type. You must create a new resource.

See the following mapping of deprecated fields to their new OTLP-based counterparts:

| Deprecated Field | Migration Action |
|:--:|:--:|
| spec.output.http or spec.output.custom | Required. Replace with spec.output.otlp. |
| spec.filters                           | Rewrite the custom Fluent Bit filters using transform or filter expressions.|
| spec.variables and spec.files          | These fields were used by custom filters. This functionality is now handled by transform or filter expressions.|
| spec.input.runtime.dropLabels      | This field is no longer used. Configure label enrichment in the central Telemetry resource.|
| spec.input.runtime.keepAnnotations | This functionality is not supported with the OTLP output and cannot be migrated.|

## Procedure

1. Identify deprecated fields in your LogPipeline.

   If your LogPipeline uses the `http` or `custom` output, you must migrate it to the OTLP stack and replace the deprecated fields:

    ```yaml
    apiVersion: telemetry.kyma-project.io/v1beta1
    kind: LogPipeline
    metadata:
      name: my-http-pipeline
    spec:
      input:
        runtime:             # OTLP supports the runtime input, but you must replace the dropLabels and keepAnnotation flags
          dropLabels: true       # Configure label enrichment centrally
          keepAnnotations: true  # no longer supported
      filters:
        custom: |                # Replace with a transform or filter expression
          ...
      variables:                 # Used for custom filters, replace with a transform or filter expression
        - name: myVar
          value: myValue
      files:                     # Used for custom filters, replace with transform or filter expressions
        - name: myFile
          value: |
            ...
      output:
        http:                    # Switch to OTLP
          endpoint:
            value: "my-backend:4317"
        custom: |                # Switch to OTLP
          ...
    ```

1. Create a new LogPipeline that uses the `otlp` output.

    Pay special attention to the following settings (for details, see [Integrate With Your OTLP Backend](./README.md)):

    * Endpoint URL: Use the OTLP-specific ingestion endpoint from your observability backend. This URL is different from the one used for the legacy `http` output.
    * Protocol: The `otlp` output defaults to the gRPC protocol. If your backend uses HTTP, you must include the protocol in the endpoint URL (for example, https://my-otlp-http-endpoint:4318).
    * Authentication: The OTLP endpoint often uses different credentials or API permissions than your previous log ingestion endpoint. Verify that your credentials have the necessary permissions for OTLP log ingestion.

    ```yaml
    apiVersion: telemetry.kyma-project.io/v1beta1
    kind: LogPipeline
    metadata:
      name: my-otlp-pipeline
    spec:
      output:
        otlp:
          endpoint:
            value: "my-backend:4317"
    ```

1. (Optional) If your old pipeline uses `custom` filters, rewrite them using the [OpenTelemetry Transformation Language (OTTL)](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md) and add them to your new LogPipeline (see [Transform and Filter with OTTL](./../filter-and-process/ottl-transform-and-filter/README.md)).
  
   Example: You want to replace a legacy Fluent Bit filter that dropped health checks and added a **tenant** attribute:

   ```yaml
   apiVersion: telemetry.kyma-project.io/v1beta1
   kind: LogPipeline
   metadata:
     name: my-http-pipeline
   spec:
     filter:
       - custom: |
         Name    grep
         Exclude path /healthz/ready
       - custom: |
         Name    record_modifier
         Record  tenant myTenant
     output:
       http:
         ...
   ```

   In your new OTLP pipeline, use the `filter` and `transform` sections with OTTL expressions:

   ```yaml
   apiVersion: telemetry.kyma-project.io/v1beta1
   kind: LogPipeline
   metadata:
     name: my-otlp-pipeline
   spec:
     transform:
       - conditions:
         - log.attributes["tenant"] == ""
         statements:
         - set(log.attributes["tenant"], "myTenant")
     filter:
       conditions:
         - log.attributes["path"] == "/healthz/ready"
     output:
       otlp:
         ...
   ```

1. (Optional) To enrich logs with Pod labels, configure the central Telemetry resource ([Telemetry CRD](https://kyma-project.io/#/telemetry-manager/user/resources/01-telemetry)).

   In contrast to a Fluent Bit LogPipeline, the `otlp` output doesn't automatically add all Pod labels. To continue enriching logs with specific labels, you must explicitly enable it in the spec.enrichments.extractPodLabels field.

   > [!NOTE]
   > Enrichment with Pod annotations is no longer supported.

1. Deploy the new LogPipeline:

   ```shell
   kubectl apply -f logpipeline.yaml
   ```

1. Check that the new LogPipeline is healthy:

   ```shell
   kubectl get logpipeline my-otlp-pipeline
   ```

1. Check your observability backend to confirm that log data is arriving.

1. Delete the old LogPipeline:

   ```shell
   kubectl delete logpipeline my-http-pipeline
   ```

## Result

Your cluster now sends logs exclusively through your new OTLP-based LogPipeline. Your filter and enrichment logic is preserved.
