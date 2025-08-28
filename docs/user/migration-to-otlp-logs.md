# Migration of LogPipelines from HTTP/Custom Output to OTLP Output

## Overview

This guide explains how to migrate existing LogPipelines using HTTP or custom outputs to the new OTLP output format.

## Motivation

- Use the vendor-neutral OTLP protocol opening up a broad variety of supported backends
- Have the same resource attributes used on your log data as on traces and metrics, allowing cross-correlation between signal types
- Support for OTLP push-based collection of log data
- New features will be added for the OTel Collector based technology stack only, so be future-ready

## Prerequisites

- Telemetry module is enabled and existing LogPipeline with HTTP or custom output are in use

## Important Notes

- Direct modification of existing LogPipelines to OTLP output is not supported, a new LogPipeline must be created with OTLP output
- A migration without data loss is possible by establishing a OTLP based LogPipeline before removing the existing one. They can exist in parallel.

## Basic Migration Steps

### Enable OTLP Ingestion at your Backend

The modernized LogPipeline API is based on OTLP fully and supports OTlP only as output protocol. With that, it is essential to switch to an OTLP based ingestion on your backend. Assure that this is supported natively and identify the OTLP endpoints to be configured in the new LogPipeline. Usually the OTLP endpoints are different to the ones used before. Also check if GRPC is supported, otherwise you will need to configure the HTTP protocol explicitly. For authentication potentially the same approach could be used, usually different permissions are required to inject in OTLP.

In case native OTLP is not supported, you will need to run a custom OTel Collector as gateway between the Telemetry module and the target backend.

### 1. Create New LogPipeline

Create a new LogPipeline manifest with an OTLP output. Use the identified OTLP endpoint and configure it as endpoint. For details on the different configuration options please see [Telemetry Pipeline OTLP Output](./pipelines/otlp-output.md). Hereby, verify the used protocol and authentication method.

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

### 2. Deploy Both Pipelines

Deploy the new OTLP pipeline alongside your existing pipeline:

```bash
kubectl apply -f logpipeline.yaml
```

### 3. Verify the New Pipeline

Check that logs are flowing through the new pipeline by checking the status:

```bash
kubectl get logpipeline my-otlp-pipeline
```

### 4. Remove Old Pipeline

After verification, remove the old pipeline:

```bash
kubectl delete logpipeline my-old-pipeline
```

## Custom Transformation and Filtering

The old LogPipeline offers the `filters` element (in combination with the `files` and `variables` attributes) which supports custom transformation and filtering of the log payload and the proprietary metadata attributes, based on a FLuent-Bit native configuration, requiring advanced knowledge in the available filter plugins and there concrete usage.

With the introduction of the OTLP support, a new powerful transform and filter API got introduced which fully rely on one well-defined langauge [OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md). With that, you can transform or extend the existing resource and log attributes, transform the log data and perform advanced filtering.

With that, an old LogPipeline like this:

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

could be transformed to a new LogPipeline:

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

There is no golden rule to re-write these transform and filter rules, so please have a look at the documentation of the [Transform & Filter](./pipelines/enrichment.md) API

## Selective Enrichement

The old LogPipeline is automatically enriching all labels of the source Pod as metadata. There is an option `input.application.dropLabels` to disable the enrichment fully.
With the new LogPipeline, the enrichment is disabled by default and can be enabled selectively by label names or groups. The enrichemnt can be configured centrally only, not individually per pipeline, and can be found in the Telemetry resource. For details see [Enrichment](./pipelines/enrichment.md)

The enrichment of annotations of a source Pod via the flag `input.application.keepAnnotations` is not supported anymore.
