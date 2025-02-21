# 18. OTel Logging Agent Setup

Date: 2025-02-18

## Status
Proposed

## Context
This document builds upon the previous ADR [17. Fault Tolerant OTel Logging Setup](./017-fault-tolerant-otel-logging-setup.md).

The goal of this document is to have an implementation-ready log agent configuration. The following points are considered:
1. Given that every LogPipeline gets its own OTelPipeline, what mechanism will be used for the namespace filtering? Dedicated vs shared tail receiver.
2. The agent will have no explicit batching mechanism configured.
3. The agent will have no sending queue mechanism configured.
4. Agent will send logs directly to the OTLP exporter (no gateway involved).
5. Relevant gateway logic will be copied to the agent

### Explored solutions to the namespace filtering problem:

The following solutions have been explored and proposed for the namespace filtering issue:

![Namespace Filtering - Explored Solutions](../assets/logs-otel-agent.drawio.svg)

Although **solution 1.2** is more performance efficient, its implementation complexity led the team to choose **solution 2** instead.

Another consideration could be the number of LogPipelines generally used on each active cluster. This can be probed with the following PromQL query:

```
count_values("LogPipelines/shoot", count by (shoot) (count by (shoot,name) (kube_customresource_telemetry_logpipeline_condition)))
```

Leading to the following results for the production environment (as of this writing):
| LogPipelines/shoot | shoots |  ~%   |
| :----------------: | :----: | :---: |
|         1          |   53   | 18.8% |
|         2          |  206   |  73%  |
|         3          |   9    | 3.2%  |
|         4          |   13   | 4.6%  |
|         5          |   1    | 0.4%  |

This means that some performance drawbacks due to duplicated tailing can still be expected, in going with **solution 2**. But this is however highly dependent on the pipelines' configuration. With this information at hand, we know what to optimize for, given that the vast majority of clusters (i.e. 91.8%) have 2 or fewer LogPipelines configured. This results in the same log files being tailed a maximum of two times (in the worst case scenario) for >90% of the clusters.

## Decision

Given the points considered above, the following LogPipeline agent configuration template is proposed:

``` yaml
exporters:
    otlp/pipeline1:
        # ...
        sending_queue:
            enabled: false
    otlp/pipeline2:
        # ...
        sending_queue:
            enabled: false

extensions:
    file_storage:
        directory: /var/lib/otelcol
    health_check:
        endpoint: ${env:MY_POD_IP}:13133
    pprof:
        endpoint: 127.0.0.1:1777

processors:
    memory_limiter:
        check_interval: 5s
        limit_percentage: 80
        spike_limit_percentage: 25
    transform/set-instrumentation-scope-runtime:
        error_mode: ignore
        metric_statements:
            - context: scope
              statements:
              - set(version, "main")
              - set(name, "io.kyma-project.telemetry/runtime")
    k8sattributes: # previous gateway processor
          auth_type: serviceAccount
          passthrough: false
          extract:
              metadata:
                  - k8s.pod.name
                  - k8s.node.name
                  - k8s.namespace.name
                  - k8s.deployment.name
                  - k8s.statefulset.name
                  - k8s.daemonset.name
                  - k8s.cronjob.name
                  - k8s.job.name
              labels:
                  - from: pod
                    key: app.kubernetes.io/name
                    tag_name: kyma.kubernetes_io_app_name
                  - from: pod
                    key: app
                    tag_name: kyma.app_name
          pod_association:
              - sources:
                  - from: resource_attribute
                    name: k8s.pod.ip
              - sources:
                  - from: resource_attribute
                    name: k8s.pod.uid
              - sources:
                  - from: connection
    resource/insert-cluster-name: # previous gateway processor
        attributes:
            - action: insert
            key: k8s.cluster.name
            value: <CLUSTER_NAME> # cluster name

receivers:
    filelog/pipeline1:
        exclude:
        - /var/log/pods/kyma-system_telemetry-log-agent*/*/*.log # exclude self
        - /var/log/pods/kyma-system_telemetry-fluent-bit*/*/*.log # exclude FluentBit
        include:
        - /var/log/pods/*/*/*.log
        # +inclusions/exclusions of LogPipeline 1
        include_file_name: false
        include_file_path: true
        operators:
            - type: container
              id: container-parser
              add_metadata_from_filepath: true
              format: containerd
            - from: attributes.stream
              if: attributes.stream != nil
              to: attributes["log.iostream"]
              type: move
            - if: body matches "^{.*}$"
              parse_from: body
              parse_to: attributes
              type: json_parser
            - from: body
              to: attributes.original
              type: copy
            - from: attributes.message
              if: attributes.message != nil
              to: body
              type: move
            - from: attributes.msg
              if: attributes.msg != nil
              to: body
              type: move
            - if: attributes.level != nil
              parse_from: attributes.level
              type: severity_parser
        retry_on_failure:
            enabled: true
        start_at: beginning
        storage: file_storage
    filelog/pipeline2:
        # ...
        # +inclusions/exclusions of LogPipeline 2

service:
    extensions:
        - health_check
        - pprof
        - file_storage
    pipelines:
        logs/pipeline1:
            exporters:
            - otlp/pipeline1
            processors:
            - memory_limiter
            - transform/set-instrumentation-scope-runtime
            receivers:
            - filelog/pipeline1
        logs/pipeline2:
            exporters:
            - otlp/pipeline2
            processors:
            - memory_limiter
            - transform/set-instrumentation-scope-runtime
            receivers:
            - filelog/pipeline2
        # ...
    telemetry:
    # ...
```