---
project: telemetry-manager
collector-image-env: ENV_OTEL_COLLECTOR_IMAGE
go-mod-path: go.mod
config-dirs: internal/otelcollector/config
validator-paths: internal/validators/ottl
---

## Components

| Name | Type | Notes |
| --- | --- | --- |
| otlp | receiver | OTLP gRPC/HTTP ingestion in the gateway |
| prometheusreceiver | receiver | Scrapes Prometheus endpoints in the metric agent |
| kubeletstatsreceiver | receiver | Kubernetes node/pod/container runtime metrics |
| k8sclusterreceiver | receiver | Kubernetes cluster-level metrics and object metadata |
| kymastatsreceiver | receiver | Custom receiver in the kyma-otel-collector distribution |
| k8sattributesprocessor | processor | Enriches telemetry with pod/node/namespace metadata; uses explicit `TagName`s for all label extractions — v0/v1 semconv flip is a no-op |
| filterprocessor | processor | User-facing filter conditions exposed via `FilterSpec` CRD field |
| transformprocessor | processor | User-facing OTTL statements exposed via `TransformSpec` CRD field; also used internally for service enrichment and cluster attribute injection |
| batch | processor | Batching before export; configured with explicit queue size |
| memory_limiter | processor | Memory cap on all pipeline agents |
| resource | processor | Static resource attribute manipulation |
| attributes | processor | Attribute-level transformations |
| istio_noise_filter | processor | Custom processor in the kyma-otel-collector distribution |
| otlp | exporter | OTLP gRPC/HTTP export to user-configured backends |
| routing | connector | Routes signals to per-pipeline OTLP exporters |
| forward | connector | Forwards signals within a pipeline fan-out |

## OTTL exposure

User-supplied OTTL statements are validated in-process via the admission webhook at `internal/validators/ottl`. The validator builds a `ParserCollection` using `pkg/ottl` and the processors' `Default*Functions()` helpers, compiled directly against the collector Go API. Any Go-API break in `pkg/ottl` surfaces here as a compile error rather than a runtime error, but OTTL **language/grammar** changes (stricter parsing, removed functions, changed semantics) surface as a behavior change in what statements users are allowed to submit.

## Key attributes

Attribute names the project depends on by string in filters, routing rules, transform statements, or documentation:

- `k8s.pod.name`
- `k8s.pod.uid`
- `k8s.pod.ip`
- `k8s.node.name`
- `k8s.namespace.name`
- `k8s.deployment.name`
- `k8s.statefulset.name`
- `k8s.daemonset.name`
- `k8s.cronjob.name`
- `k8s.job.name`
- `k8s.pod.label.<key>` (singular v1 form — explicit `TagName` used in k8sattributes config)
- `k8s.cluster.name`
- `k8s.cluster.uid`
- `service.name`
- `service.version`
- `service.namespace`
- `service.instance.id`
- `cloud.region`
- `cloud.availability_zone`
- `host.type`
- `host.arch`
- `kyma.otel.annotation.service.name` (temp attribute used during service enrichment)
- `kyma.otel.annotation.service.version` (temp attribute used during service enrichment)
- `kyma.input.name`

## Notes

- The collector is **not** run from this repo — it is built as a custom distribution in the `opentelemetry-collector-components` repository. That is where `kymastatsreceiver` and `istio_noise_filter` live and where Go-API breaks in those custom components would surface. Grep that repo separately when evaluating collector bumps.
- The `sending_queue` / `batch` exporter config is built via `internal/otelcollector/config/common/otlpexporter_config_builder.go`. No `partition` or `idle_timeout` is set explicitly — upstream default changes to those fields apply.
- OTTL `set()` statements that source from potentially-absent resource attributes (e.g. `set(service.name, annotation_attr)`) are guarded with `where IsNotNil AND != ""` — `ottl.set.allowNil` being on by default is not a risk for current code.
- Golden files for generated configs live under `internal/otelcollector/config/*/testdata/`. Run `make update-golden-files` after a bump and diff carefully — behavioral OTTL/prometheus/semconv changes are most likely to shift these.
