# Migrate Your LogPipeline From HTTP to OTLP

To use the OpenTelemetry Protocol (OTLP) for sending logs, you must migrate your `LogPipeline` from the `http` or `custom` output to the `otlp` output. With OTLP, you can correlate logs with traces and metrics, collect logs pushed directly from applications, and use features available only for the OTLP-based stack.

## Prerequisites

* You have an active Kyma cluster with the Telemetry module added.
* You have one or more `LogPipeline` resources that use the `http` or `custom` output.
* Your observability backend has an OTLP ingestion endpoint.
  If your backend doesn't support OTLP natively, you must run a custom OTel Collector as gateway between the Telemetry module and the target backend.

## Context

When you want to migrate to the `otlp` output, create a new `LogPipeline`. To prevent data loss, run it in parallel with your existing pipeline. After verifying that the new pipeline works correctly, you can delete the old one.

You can't modify an existing `LogPipeline` to change its output type. You must create a new resource.

## Procedure

1. Create a new `LogPipeline` that uses the `otlp` output.

    Pay special attention to the following settings (for details, see [Integrate With Your OTLP Backend](migration-to-otlp-logs.md):

    * Endpoint URL: Use the OTLP-specific ingestion endpoint from your observability backend. This URL is different from the one used for the legacy `http` output.
    * Protocol: The `otlp` output defaults to the gRPC protocol. If your backend uses HTTP, you must include the protocol in the endpoint URL (for example, https://my-otlp-http-endpoint:4318).
    * Authentication: The OTLP endpoint often uses different credentials or API permissions than your previous log ingestion endpoint. Verify that your credentials have the necessary permissions for OTLP log ingestion.

    ```yaml
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: LogPipeline
    metadata:
      name: my-otlp-pipeline
    spec:
      output:
        otlp:
          endpoint:
            value: "my-backend:4317"
    ```

2. (Optional) If your old pipeline uses `custom` filters, rewrite them using the OpenTelemetry Transformation Language ([OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md)) and add them to your new LogPipeline.
  
   Example: You want to replace a legacy Fluent Bit filter that dropped health checks and added a `tenant` attribute:

   ```yaml
   apiVersion: telemetry.kyma-project.io/v1alpha1
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
   apiVersion: telemetry.kyma-project.io/v1alpha1
   kind: LogPipeline
   metadata:
     name: my-http-pipeline
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

3. (Optional) To enrich logs with labels, configure the central Telemetry resource.

   In contrast to a Fluent Bit LogPipeline, the `otlp` output doesn't automatically add all Pod labels. To continue enriching logs with specific labels, you must explicitly enable it in the spec.enrichments.extractPodLabels field.

   > **Note:** Enrichment with Pod annotations is no longer supported.

4. Deploy the new `LogPipeline`:

   ```shell
   kubectl apply -f logpipeline.yaml
   ```

5. Check that the new `LogPipeline` is healthy:

   ```shell
   kubectl get logpipeline my-otlp-pipeline
   ```

6. Check your observability backend to confirm that log data is arriving.

7. Delete the old `LogPipeline`:

   ```shell
   kubectl delete logpipeline my-old-pipeline
   ```

## Result

Your cluster now sends logs exclusively through your new OTLP-based `LogPipeline`. Your filter and enrichment logic is preserved.
