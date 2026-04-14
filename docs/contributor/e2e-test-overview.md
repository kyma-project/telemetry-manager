# E2E Test Overview

This document provides a comprehensive overview of all end-to-end and integration tests in the Telemetry Manager project.

## Test Suites at a Glance

| Suite | Location | Make Target | Description |
|---|---|---|---|
| Traces | [test/e2e/traces/](../../test/e2e/traces/) | `make e2e-traces` | TracePipeline E2E tests |
| Logs | [test/e2e/logs/](../../test/e2e/logs/) | `make e2e-logs` | LogPipeline E2E tests (agent, FluentBit, gateway, shared, misc) |
| Metrics | [test/e2e/metrics/](../../test/e2e/metrics/) | `make e2e-metrics` | MetricPipeline E2E tests (agent, gateway, shared, misc) |
| Misc | [test/e2e/misc/](../../test/e2e/misc/) | `make e2e-misc` | Cross-cutting concerns (RBAC, Telemetry CR, overrides, labels) |
| Upgrade | [test/e2e/upgrade/](../../test/e2e/upgrade/) | `make e2e-upgrade` | Pipeline survival across manager upgrades and CRD migrations |
| Self-Monitor | [test/selfmonitor/](../../test/selfmonitor/) | `make selfmonitor-test` | Self-monitoring: healthy baselines, backpressure, and outage detection |
| Integration (Istio) | [test/integration/istio/](../../test/integration/istio/) | `make integration-test` | Istio service mesh integration (access logs, metrics, trace routing) |

## Traces ([test/e2e/traces/](../../test/e2e/traces/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestSinglePipeline](../../test/e2e/traces/single_pipeline_test.go) (grpc, http) | 1 | OTLP output with gRPC or HTTP protocol | Basic single pipeline delivers traces and emits OTel collector metrics |
| [TestSinglePipelineV1Alpha1](../../test/e2e/traces/single_pipeline_v1alpha1_test.go) | 1 | OTLP output (v1alpha1 API), gRPC | Backward compatibility: v1alpha1 TracePipeline delivers traces and emits metrics |
| [TestEndpointInvalid](../../test/e2e/traces/endpoint_invalid_test.go) | 4 | OTLP output with invalid endpoints (quoted URL, invalid secret, missing port, invalid port) | Invalid endpoints cause `ConfigurationGenerated=False/EndpointInvalid`; missing port with HTTP is accepted |
| [TestEndpointWithPathValidation](../../test/e2e/traces/endpoint_with_path_validation_test.go) | 5 | OTLP output with gRPC/HTTP protocol combinations with/without path | Webhook rejects `path` with gRPC protocol; accepts path with HTTP |
| [TestEnrichmentValuesEmpty](../../test/e2e/traces/enrichment_values_empty_test.go) | 1 | OTLP output; trace generator sends empty resource attributes | Enrichment processors populate empty cloud/k8s/service.name attributes; kyma.* attributes dropped |
| [TestEnrichmentValuesPredefined](../../test/e2e/traces/enrichment_values_predefined_test.go) | 1 | OTLP output; trace generator sends predefined resource attributes | Enrichment preserves predefined attribute values; drops kyma.* internal attributes |
| [TestExtractLabels](../../test/e2e/traces/extract_labels_test.go) | 1 | OTLP output; Telemetry CR with `ExtractPodLabels` (exact key + key prefix) | Pod labels matching exact key/prefix extracted as `k8s.pod.label.*` attributes |
| [TestFilter](../../test/e2e/traces/filter_test.go) | 1 | OTLP output; Transform sets attribute; Filter drops matching spans | Filter drops spans matching the condition |
| [TestFilterInvalid](../../test/e2e/traces/filter_invalid_test.go) | 1 | OTLP output; Filter with invalid condition (missing context prefix) | Webhook rejects pipeline with invalid filter condition |
| [TestMTLS](../../test/e2e/traces/mtls_test.go) | 1 | OTLP HTTPS; full mTLS (CA + client cert + key) | Traces delivered successfully over mTLS |
| [TestMTLSAboutToExpireCert](../../test/e2e/traces/mtls_about_to_expire_cert_test.go) | 1 | OTLP HTTPS; mTLS with about-to-expire cert | Pipeline healthy with `TLSCertificateAboutToExpire` warning; traces still delivered |
| [TestMTLSExpiredCert](../../test/e2e/traces/mtls_expired_cert_test.go) | 1 | OTLP HTTPS; mTLS with shortly-expiring cert | Transitions from about-to-expire warning to `TLSCertificateExpired` error |
| [TestMTLSInvalidCA](../../test/e2e/traces/mtls_invalid_ca_test.go) | 1 | OTLP HTTPS; mTLS with invalid CA | `TLSConfigurationInvalid` condition; Telemetry state=Warning |
| [TestMTLSInvalidCert](../../test/e2e/traces/mtls_invalid_cert_test.go) | 1 | OTLP HTTPS; mTLS with invalid client cert | `TLSConfigurationInvalid` condition; Telemetry state=Warning |
| [TestMTLSCertKeyPairDontMatch](../../test/e2e/traces/mtls_cert_key_pair_dont_match_test.go) | 1 | OTLP HTTPS; mTLS with mismatched cert/key pair | `TLSConfigurationInvalid` condition; Telemetry state=Warning |
| [TestMultiPipelineBroken](../../test/e2e/traces/multi_pipeline_broken_test.go) | 2 | 1 healthy + 1 broken (missing secret ref) OTLP pipelines | Healthy pipeline delivers traces; broken pipeline has `ReferencedSecretMissing` |
| [TestMultiPipelineFanout](../../test/e2e/traces/multi_pipeline_fanout_test.go) | 2 | 2 OTLP pipelines to different backends | Traces fanned out to both backends |
| [TestMultiPipelineMaxPipeline](../../test/e2e/traces/multi_pipeline_max_pipeline_test.go) (normal, experimental) | max+1 | Multiple OTLP pipelines | Normal: exceeding max → `MaxPipelinesExceeded`; Experimental: unlimited allowed |
| [TestNoisyFilters](../../test/e2e/traces/noisy_span_filter_test.go) | 1 | OTLP output; 11 span generators with Istio/system attributes | Built-in noisy span filtering drops internal/system spans; regular spans delivered |
| [TestOAuth2](../../test/e2e/traces/oauth2_test.go) | 1 | OTLP HTTPS; TLS (CA); OAuth2 (client_credentials) with OIDC mock | Traces delivered via OAuth2-authenticated connection |
| [TestRejectTracePipelineCreation](../../test/e2e/traces/reject_creation_test.go) (14 subtests) | 1 each | Various invalid configs: no output, value+valueFrom, missing secretKeyRef, gRPC+path, invalid protocol, missing basic auth, TLS missing key/cert, invalid OAuth2 | Webhook rejects invalid pipeline specs with correct error messages |
| [TestResources](../../test/e2e/traces/resources_test.go) | 1 | OTLP output with endpoint from secret | Gateway resources created; cleaned up when pipeline becomes non-reconcilable |
| [TestSecretMissing](../../test/e2e/traces/secret_missing_test.go) | 1 | OTLP output with endpoint from non-existent secret | `ReferencedSecretMissing` condition; creating secret heals pipeline |
| [TestSecretRotation](../../test/e2e/traces/secret_rotation_test.go) | 1 | OTLP output with endpoint from secret (initially wrong) | Traces not delivered with wrong value; delivered after secret update |
| [TestServiceEnrichment](../../test/e2e/traces/service_enrichment_test.go) | 1 | OTLP output; OTel service enrichment strategy annotation | service.name/namespace/version/instance.id enrichment for various pod states |
| [TestServiceName](../../test/e2e/traces/service_name_test.go) | 1 | OTLP output; pods with undefined/unknown service names | Legacy service name enrichment: undefined names replaced with pod name |
| [TestTransform](../../test/e2e/traces/transform_test.go) (with-where, cond-and-stmts, infer-context) | 1 each | OTLP output; various OTTL transform statements | OTTL transform statements execute correctly across contexts |
| [TestTransformInvalid](../../test/e2e/traces/transform_invalid_test.go) | 1 | OTLP output; Transform with invalid statement (typo) | Webhook rejects pipeline with invalid transform |

## Logs

### Agent-Specific ([test/e2e/logs/agent/](../../test/e2e/logs/agent/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestInstrumentationScope](../../test/e2e/logs/agent/instrumentation_scope_test.go) | 1 | OTLP output; runtime input with namespace selector | Logs have correct instrumentation scope name and version |
| [TestSeverityParser](../../test/e2e/logs/agent/severity_parser_test.go) | 1 | OTLP output; runtime input with namespace selector | Severity parsing from `level`/`log.level` attributes; parsed attributes removed |
| [TestTraceParser](../../test/e2e/logs/agent/trace_parser_test.go) | 1 | OTLP output; runtime input with namespace selector | Trace context parsing from `trace_id`/`span_id`/`traceparent` into OTel fields |

### FluentBit-Specific ([test/e2e/logs/fluentbit/](../../test/e2e/logs/fluentbit/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestAppName](../../test/e2e/logs/fluentbit/app_name_test.go) | 1 | HTTP output; container selector; FluentBit backend | `app_name` set from `app.kubernetes.io/name` or `app` labels with precedence |
| [TestBasePayloadWithHTTPOutput](../../test/e2e/logs/fluentbit/base_payload_with_http_test.go) | 1 | HTTP output; namespace+container selectors; FluentBit backend | FluentBit HTTP payload has expected attributes: timestamp, K8s metadata, cluster_identifier |
| [TestCustomClusterName](../../test/e2e/logs/fluentbit/custom_cluster_name_test.go) | 1 | HTTP output; custom Telemetry CR cluster name enrichment; FluentBit backend | Custom cluster name applied via `cluster_identifier` attribute |
| [TestCustomFilterAllowed](../../test/e2e/logs/fluentbit/custom_filter_test.go) | 1 | HTTP output with dedot; 2 custom grep filters (include/exclude) | Custom grep filters work; unsupported mode flagged |
| [TestCustomFilterDenied](../../test/e2e/logs/fluentbit/custom_filter_test.go) | 1 | HTTP output with dedot; unsupported custom filter | Unsupported custom filter denied (TLS config invalid condition) |
| [TestCustomOutput](../../test/e2e/logs/fluentbit/custom_output_test.go) | 1 | Custom output (HTTP format); FluentBit backend | Custom HTTP output delivers logs; unsupported mode flagged |
| [TestCustomOutputDenied](../../test/e2e/logs/fluentbit/custom_output_test.go) | 1 | Custom output (random); FluentBit backend | Unsupported custom output denied |
| [TestDedot](../../test/e2e/logs/fluentbit/dedot_test.go) | 1 | HTTP output with dedot enabled; container selector | Dots in label keys replaced with underscores |
| [TestKeepAnnotations](../../test/e2e/logs/fluentbit/keep_annotation_test.go) | 1 | HTTP output; keepAnnotations=true, dropLabels=true | Annotations kept, labels dropped in output |

### Gateway-Specific ([test/e2e/logs/gateway/](../../test/e2e/logs/gateway/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestEnrichmentValuesEmpty](../../test/e2e/logs/gateway/enrichment_values_empty_test.go) (gateway, gateway-experimental) | 1 each | OTLP output; namespace selector; empty resource attributes | Enrichment fills empty cloud/k8s attributes; drops kyma.* |
| [TestEnrichmentValuesPredefined](../../test/e2e/logs/gateway/enrichment_values_predefined_test.go) (gateway, gateway-experimental) | 1 each | OTLP output; namespace selector; predefined resource attributes | Enrichment preserves predefined values; drops kyma.* |

### Misc / Validation ([test/e2e/logs/misc/](../../test/e2e/logs/misc/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestEndpointInvalid_OTel](../../test/e2e/logs/misc/endpoint_invalid_test.go) | 2 | OTLP output with invalid endpoint (value + secret) | Invalid endpoints get `EndpointInvalid` condition |
| [TestEndpointInvalid_FluentBit](../../test/e2e/logs/misc/endpoint_invalid_test.go) | 2 | HTTP output with invalid host (value + secret) | Same for FluentBit pipelines |
| [TestFilterInvalid](../../test/e2e/logs/misc/filter_invalid_test.go) | 1 | OTLP output; filter with invalid condition | Webhook rejects invalid filter condition |
| [TestMTLSCertKeyDontMatch_OTel](../../test/e2e/logs/misc/mtls_cert_key_pair_dont_match_test.go) | 1 | OTLP HTTPS; mTLS with mismatched cert/key | TLS configuration invalid condition |
| [TestMTLSCertKeyDontMatch_FluentBit](../../test/e2e/logs/misc/mtls_cert_key_pair_dont_match_test.go) | 1 | HTTP output; mTLS with mismatched cert/key | Same for FluentBit |
| [TestMTLSExpiredCert_OTel](../../test/e2e/logs/misc/mtls_expired_cert_test.go) | 1 | OTLP HTTPS; mTLS with shortly-expiring cert | Cert lifecycle: about-to-expire → expired |
| [TestMTLSExpiredCert_FluentBit](../../test/e2e/logs/misc/mtls_expired_cert_test.go) | 1 | HTTP output; mTLS with shortly-expiring cert | Same for FluentBit |
| [TestMTLSInvalidCA_OTel](../../test/e2e/logs/misc/mtls_invalid_ca_test.go) | 1 | OTLP HTTPS; mTLS with invalid CA | TLS configuration invalid for bad CA |
| [TestMTLSInvalidCA_FluentBit](../../test/e2e/logs/misc/mtls_invalid_ca_test.go) | 1 | HTTP output; mTLS with invalid CA | Same for FluentBit |
| [TestMTLSInvalidCert_OTel](../../test/e2e/logs/misc/mtls_invalid_cert_test.go) | 1 | OTLP HTTPS; mTLS with invalid client cert | TLS configuration invalid for bad cert |
| [TestMTLSInvalidCert_FluentBit](../../test/e2e/logs/misc/mtls_invalid_cert_test.go) | 1 | HTTP output; mTLS with invalid client cert | Same for FluentBit |
| [TestRejectLogPipelineCreation](../../test/e2e/logs/misc/reject_creation_test.go) (~30 subtests) | 1 each | Various invalid configs (no output, value+valueFrom, missing secrets, gRPC+path, invalid protocol, bad auth, bad TLS, bad OAuth2, invalid selectors, incompatible output/input combos) | Webhook rejects all invalid LogPipeline configurations |
| [TestTransformInvalid](../../test/e2e/logs/misc/transform_invalid_test.go) | 1 | OTLP output; invalid OTTL statement | Webhook rejects invalid transform |
| [TestVersionConversion](../../test/e2e/logs/misc/version_conversion_test.go) | 2 | v1alpha1 HTTP output + v1beta1 HTTP output | CRD version conversion: v1alpha1 ↔ v1beta1 |

### Shared (agent + gateway + FluentBit subtests) ([test/e2e/logs/shared/](../../test/e2e/logs/shared/))

Tests in this directory run as subtests for multiple backends: OTel agent, OTel gateway, OTel gateway-experimental, and FluentBit.

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestSinglePipeline_OTel](../../test/e2e/logs/shared/single_pipeline_test.go) (agent/grpc, agent/http, gateway/grpc, gateway/http, gateway-experimental/grpc, gateway-experimental/http) | 1 each | OTLP output with gRPC or HTTP protocol; namespace selector | Basic single pipeline log delivery over both protocols |
| [TestSinglePipeline_FluentBit](../../test/e2e/logs/shared/single_pipeline_test.go) | 1 | HTTP output; FluentBit backend | Basic FluentBit pipeline log delivery |
| [TestSinglePipelineV1Alpha1_OTel](../../test/e2e/logs/shared/single_pipeline_v1alpha1_test.go) (agent, gateway, gateway-experimental) | 1 each | v1alpha1 OTLP output; gRPC; insecure TLS | v1alpha1 API backward compatibility |
| [TestSinglePipelineV1Alpha1_FluentBit](../../test/e2e/logs/shared/single_pipeline_v1alpha1_test.go) | 1 | v1alpha1 HTTP output; TLS disabled | v1alpha1 FluentBit compatibility |
| [TestMetricsEndpoint_OTel](../../test/e2e/logs/shared/check_metrics_endpoint_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output; runtime or OTLP input | OTel Collector metrics endpoint emits metrics |
| [TestMetricsEndpoint_FluentBit](../../test/e2e/logs/shared/check_metrics_endpoint_test.go) | 1 | HTTP output; FluentBit backend | FluentBit metrics endpoint emits Prometheus metrics |
| [TestContainerSelector_OTel](../../test/e2e/logs/shared/container_selector_test.go) | 2 | OTLP output; container include on pipeline 1, exclude on pipeline 2 | Container include/exclude selectors route logs correctly |
| [TestContainerSelector_FluentBit](../../test/e2e/logs/shared/container_selector_test.go) | 2 | HTTP output; container include/exclude | Same for FluentBit |
| [TestDisabledInput_OTel](../../test/e2e/logs/shared/disabled_input_test.go) | 1 | OTLP output; runtime input disabled, OTLP input disabled | No DaemonSet when runtime disabled; no logs when OTLP disabled |
| [TestDisabledInput_FluentBit](../../test/e2e/logs/shared/disabled_input_test.go) | 1 | HTTP output; runtime input disabled | No FluentBit DaemonSet when input disabled |
| [TestExtractLabels_OTel](../../test/e2e/logs/shared/extract_labels_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output; Telemetry CR with `extractPodLabels` (exact + prefix) | Pod label extraction into `k8s.pod.label.*` attributes |
| [TestExtractLabels_FluentBit](../../test/e2e/logs/shared/extract_labels_test.go) | 2 | HTTP output; dropLabels=true vs false; keepAnnotations=false | FluentBit label/annotation dropping behavior |
| [TestFilter_OTel](../../test/e2e/logs/shared/filter_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output; transform + filter condition | Filter conditions drop matching logs |
| [TestKeepOriginalBody_OTel](../../test/e2e/logs/shared/keep_original_body_test.go) | 2 | OTLP output; keepOriginalBody=true vs false | `log.original` attribute presence/absence; body extraction |
| [TestKeepOriginalBody_FluentBit](../../test/e2e/logs/shared/keep_original_body_test.go) | 2 | HTTP output; keepOriginalBody=true vs false | Same for FluentBit |
| [TestMTLS_OTel](../../test/e2e/logs/shared/mtls_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP HTTPS; full mTLS | Successful log delivery over mTLS |
| [TestMTLS_FluentBit](../../test/e2e/logs/shared/mtls_test.go) | 1 | HTTP output; full mTLS | Same for FluentBit |
| [TestMTLSAboutToExpireCert_OTel](../../test/e2e/logs/shared/mtls_about_to_expire_cert_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP HTTPS; mTLS with about-to-expire cert | Warning condition; logs still delivered |
| [TestMTLSAboutToExpireCert_FluentBit](../../test/e2e/logs/shared/mtls_about_to_expire_cert_test.go) | 1 | HTTP output; mTLS with about-to-expire cert | Same for FluentBit |
| [TestMultiPipelineBroken_OTel](../../test/e2e/logs/shared/multi_pipeline_broken_test.go) (agent, gateway, gateway-experimental) | 2 each | 1 healthy + 1 broken (missing secret) | Healthy pipeline delivers even with broken sibling |
| [TestMultiPipelineBroken_FluentBit](../../test/e2e/logs/shared/multi_pipeline_broken_test.go) | 2 | 1 healthy + 1 broken HTTP pipeline | Same for FluentBit |
| [TestMultiPipelineFanout_OTel](../../test/e2e/logs/shared/multi_pipeline_fanout_test.go) (agent, gateway, gateway-experimental) | 2 each | OTLP output to 2 backends | Fanout: same logs to both backends |
| [TestMultiPipelineFanout_FluentBit](../../test/e2e/logs/shared/multi_pipeline_fanout_test.go) | 2 | HTTP output to 2 FluentBit backends | Same for FluentBit |
| [TestMultiPipelineMaxPipeline](../../test/e2e/logs/shared/multi_pipeline_max_pipeline_test.go) (mixed, OTel, FluentBit) | max+1 or max+2 | Mixed HTTP + OTLP or single type | Max pipeline limit enforcement; experimental unlimited |
| [TestNamespaceSelector_OTel](../../test/e2e/logs/shared/namespace_selector_test.go) (agent, gateway, gateway-experimental) | 2 each | OTLP output; 1 include + 1 exclude namespace selector | Namespace include/exclude route logs correctly |
| [TestNamespaceSelector_FluentBit](../../test/e2e/logs/shared/namespace_selector_test.go) | 2 | HTTP output; include vs exclude namespace | Same for FluentBit |
| [TestOAuth2](../../test/e2e/logs/shared/oauth2_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP HTTPS; OAuth2 (client_credentials); TLS (CA); OIDC mock | OAuth2-authenticated log delivery |
| [TestObservedTime_OTel](../../test/e2e/logs/shared/observed_time_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output; namespace selector | Logs have non-empty timestamp and non-epoch observed timestamp |
| [TestResources_OTel](../../test/e2e/logs/shared/resources_test.go) | 1 | OTLP output (endpoint from secret); OTLP + runtime input | All expected K8s resources created and reconciled |
| [TestResources_FluentBit](../../test/e2e/logs/shared/resources_test.go) | 1 | HTTP output (host from secret) | Same for FluentBit; validates cleanup on secret deletion |
| [TestSecretMissing_OTel](../../test/e2e/logs/shared/secret_missing_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output (endpoint from missing secret) | `ReferencedSecretMissing`; heals after secret created |
| [TestSecretMissing_FluentBit](../../test/e2e/logs/shared/secret_missing_test.go) | 1 | HTTP output (host from missing secret) | Same for FluentBit |
| [TestSecretRotation_OTel](../../test/e2e/logs/shared/secret_rotation_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output (endpoint from secret, initially wrong) | Logs delivered after secret rotation |
| [TestSecretRotation_FluentBit](../../test/e2e/logs/shared/secret_rotation_test.go) | 1 | HTTP output (host from secret) | Same for FluentBit |
| [TestServiceEnrichment_OTel](../../test/e2e/logs/shared/service_enrichment_test.go) (agent, gateway) | 1 each | OTLP output; OTel service enrichment annotation; keepOriginalBody | Service attribute enrichment for various pod states |
| [TestServiceName_OTel](../../test/e2e/logs/shared/service_name_test.go) (agent, gateway, gateway-experimental) | 1 each | OTLP output; keepOriginalBody | service.name derivation from labels, pod name, workload name |
| [TestTransform_OTel](../../test/e2e/logs/shared/transform_test.go) (agent/gateway × 3 patterns) | 1 each | OTLP output; OTTL transforms: with-where, cond-and-stmts, infer-context | OTTL transform statements applied correctly |

## Metrics

### Agent-Specific ([test/e2e/metrics/agent/](../../test/e2e/metrics/agent/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestPrometheusInput](../../test/e2e/metrics/agent/prometheus_input_test.go) | 1 | Prometheus input enabled; namespace include selector; OTLP HTTP output | Prometheus scraping from annotated pods/services; no runtime metric leaks |
| [TestPrometheusInputDiagnosticMetric](../../test/e2e/metrics/agent/prometheus_input_diagnostic_metric_test.go) | 1 | Prometheus input; diagnostic metrics enabled; OTLP HTTP output | Diagnostic metrics (up, scrape_duration_seconds, etc.) delivered when enabled |
| [TestRuntimeInput](../../test/e2e/metrics/agent/runtime_input_test.go) | 3 | Pipeline A: pod/container/node/volume metrics; Pipeline B: deployment/statefulset/daemonset/job; Pipeline C: defaults; all OTLP HTTP | Comprehensive runtime input: correct metric groupings, resource attributes, volume types, scopes |
| [TestRuntimeNodeNamespace](../../test/e2e/metrics/agent/runtime_node_namespace_test.go) | 2 | Runtime input with node metrics; 1 include + 1 exclude namespace selector | Node metrics delivered for both include/exclude namespace selectors |
| [TestServiceEnrichment](../../test/e2e/metrics/agent/service_enrichment_test.go) | 1 | Runtime input; OTLP HTTP output; OTel service enrichment annotation | Service attribute enrichment for agent-collected metrics |
| [TestServiceName](../../test/e2e/metrics/agent/service_name_test.go) | 1 | Runtime input (system namespace); OTLP HTTP output | Legacy service.name set to workload name for DaemonSet/Job pods |

### Gateway-Specific ([test/e2e/metrics/gateway/](../../test/e2e/metrics/gateway/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestEnrichmentValuesEmpty](../../test/e2e/metrics/gateway/enrichment_values_empty_test.go) | 1 | OTLP input; OTLP HTTP output; telemetrygen with empty attributes | Empty attributes enriched by processors; kyma.* dropped |
| [TestEnrichmentValuesPredefined](../../test/e2e/metrics/gateway/enrichment_values_predefined_test.go) | 1 | OTLP input; OTLP HTTP output; telemetrygen with predefined attributes | Predefined attributes preserved; kyma.* dropped |
| [TestKymaInput](../../test/e2e/metrics/gateway/kyma_input_test.go) | 2 | Pipeline 1: Kyma input only; Pipeline 2: Kyma + OTLP input with namespace | Kyma input delivers kyma.resource.status metrics; OTLP input scoped per pipeline |

### Misc / Validation ([test/e2e/metrics/misc/](../../test/e2e/metrics/misc/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestDisabledInput](../../test/e2e/metrics/misc/disabled_input_test.go) | 1 | All inputs disabled (prometheus, runtime, istio, OTLP); OTLP output | Agent not deployed when all agent inputs disabled; OTLP not forwarded |
| [TestEndpointInvalid](../../test/e2e/metrics/misc/endpoint_invalid_test.go) | 3 | Invalid endpoint value, invalid secret, missing port (HTTP) | Endpoint validation: invalid → `EndpointInvalid`; missing port HTTP accepted |
| [TestFilterInvalid](../../test/e2e/metrics/misc/filter_invalid_test.go) | 1 | Filter with invalid condition; OTLP output | Webhook rejects invalid filter |
| [TestMTLSCertKeyPairDontMatch](../../test/e2e/metrics/misc/mtls_cert_key_pair_dont_match_test.go) | 1 | OTLP HTTPS; mTLS mismatched cert/key | `TLSConfigurationInvalid` condition |
| [TestMTLSExpiredCert](../../test/e2e/metrics/misc/mtls_expired_cert_test.go) | 1 | OTLP HTTPS; mTLS shortly-expiring cert | Cert lifecycle: about-to-expire → expired |
| [TestMTLSInvalidCA](../../test/e2e/metrics/misc/mtls_invalid_ca_test.go) | 1 | OTLP HTTPS; mTLS invalid CA | `TLSConfigurationInvalid` condition |
| [TestMTLSInvalidCert](../../test/e2e/metrics/misc/mtls_invalid_cert_test.go) | 1 | OTLP HTTPS; mTLS invalid client cert | `TLSConfigurationInvalid` condition |
| [TestMTLSMissingKey](../../test/e2e/metrics/misc/mtls_missing_key_test.go) | 1 | OTLP HTTP; TLS CA + cert but no key | CRD validation rejects: must define both cert and key, or neither |
| [TestRejectPipelineCreation](../../test/e2e/metrics/misc/reject_creation_test.go) (~20 subtests) | 1 each | Various invalid configs (no output, value+valueFrom, missing secrets, gRPC+path, invalid protocol, bad auth/TLS/OAuth2, invalid namespace selectors for OTLP/prometheus/istio/runtime) | Webhook rejects invalid MetricPipeline configurations |
| [TestTransformInvalid](../../test/e2e/metrics/misc/transform_invalid_test.go) | 1 | Invalid OTTL statement; OTLP output | Webhook rejects invalid transform |

### Shared (agent + gateway subtests) ([test/e2e/metrics/shared/](../../test/e2e/metrics/shared/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestSinglePipeline](../../test/e2e/metrics/shared/single_pipeline_test.go) (agent/grpc, agent/http, gateway/grpc, gateway/http) | 1 each | OTLP output with gRPC or HTTP | Basic single pipeline metrics delivery |
| [TestSinglePipelineV1Alpha1](../../test/e2e/metrics/shared/single_pipeline_v1alpha1_test.go) (agent, gateway) | 1 each | v1alpha1 OTLP output (HTTP) | Backward compatibility: v1alpha1 MetricPipeline API |
| [TestCloudProviderAttributes](../../test/e2e/metrics/shared/cloud_provider_attributes_test.go) (agent, gateway) | 1 each | Custom cluster name on Telemetry CR | Cloud provider attributes (cloud.region, host.type, etc.) present |
| [TestCustomClusterName](../../test/e2e/metrics/shared/custom_cluster_name_test.go) (agent, gateway) | 1 each | Telemetry CR cluster name override | k8s.cluster.name set to custom name |
| [TestEndpointWithPathValidation](../../test/e2e/metrics/shared/endpoint_with_path_validation_test.go) (agent, gateway) | 3 each | gRPC no path, HTTP with path, HTTP no path | Endpoint+protocol+path validation accepted |
| [TestExtractLabels](../../test/e2e/metrics/shared/extract_labels_test.go) (agent, gateway) | 1 each | Telemetry CR with `extractPodLabels` (exact + prefix) | Pod labels extracted as `k8s.pod.label.*` attributes |
| [TestFilter](../../test/e2e/metrics/shared/filter_test.go) (agent, gateway) | 1 each | Transform + filter condition; OTLP HTTP output | Filter drops metrics matching condition |
| [TestMTLS](../../test/e2e/metrics/shared/mtls_test.go) (agent, gateway) | 1 each | OTLP HTTPS; full mTLS | mTLS connectivity; metrics delivered |
| [TestMTLSAboutToExpireCert](../../test/e2e/metrics/shared/mtls_about_to_expire_cert_test.go) (agent, gateway) | 1 each | OTLP HTTPS; mTLS about-to-expire cert | Warning condition; metrics still delivered |
| [TestMultiPipelineBroken](../../test/e2e/metrics/shared/multi_pipeline_broken_test.go) (agent, gateway) | 2 each | 1 healthy + 1 broken (missing secret) | Healthy pipeline delivers with broken sibling |
| [TestMultiPipelineFanout_Agent](../../test/e2e/metrics/shared/multi_pipeline_fanout_test.go) | 2 | Pipeline 1: runtime (container only); Pipeline 2: prometheus; different backends | Runtime metrics to runtime backend; prometheus to prometheus backend |
| [TestMultiPipelineFanout_Gateway](../../test/e2e/metrics/shared/multi_pipeline_fanout_test.go) | 2 | Both OTLP input; 2 backends | Same OTLP metrics to both backends |
| [TestMultiPipelineMaxPipeline](../../test/e2e/metrics/shared/multi_pipeline_max_pipeline_test.go) (normal, experimental) | max+1 | Runtime input; OTLP output | Max limit enforcement; experimental unlimited |
| [TestNamespaceSelector](../../test/e2e/metrics/shared/namespace_selector_test.go) (agent, gateway) | 2 each | 1 include + 1 exclude; runtime+prometheus+istio or OTLP input | Namespace include/exclude selectors route metrics correctly |
| [TestOAuth2](../../test/e2e/metrics/shared/oauth2_test.go) (agent, gateway) | 1 each | OTLP HTTPS; OAuth2 (client_credentials); TLS CA | OAuth2-authenticated metrics delivery |
| [TestResources](../../test/e2e/metrics/shared/resources_test.go) | 1 | OTLP + runtime input; endpoint from secret | All expected K8s resources created and reconciled |
| [TestSecretMissing](../../test/e2e/metrics/shared/secret_missing_test.go) (agent, gateway) | 1 each | OTLP output from missing secret | `ReferencedSecretMissing`; heals after secret created |
| [TestSecretRotation](../../test/e2e/metrics/shared/secret_rotation_test.go) (agent, gateway) | 1 each | OTLP output from secret (initially wrong) | Metrics delivered after secret rotation |
| [TestTransform](../../test/e2e/metrics/shared/transform_test.go) (agent × 3, gateway × 3) | 1 each | OTTL transforms: with-where, cond-and-stmts, infer-context | Transform statements applied correctly |

## Misc ([test/e2e/misc/](../../test/e2e/misc/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestLabelAnnotation](../../test/e2e/misc/custom_label_annotation_test.go) (logs-otel, logs-fluentbit, metrics, traces) | 1 each | Custom Helm labels/annotations for manager + workloads | Custom labels/annotations propagate to Deployments, DaemonSets, and Pods |
| [TestManager](../../test/e2e/misc/manager_test.go) | 0 | N/A | Manager deployment ready; core resources (CRDs, Services, ConfigMaps, NetworkPolicy, PriorityClasses) exist |
| [TestOverrides](../../test/e2e/misc/overrides_test.go) | 3 (1 Log + 1 Metric + 1 Trace) | HTTP/OTLP outputs; overrides ConfigMap for DEBUG logging | Overrides disable pipeline/telemetry reconciliation |
| [TestRBACPermissions](../../test/e2e/misc/rbac_permissions_test.go) | 0 | 4 personas (viewer, editor, admin, telemetry-only-editor) | RBAC: viewer=read-only, editor/admin=full CRUD, telemetry-editor=no Secrets |
| [TestRBACRoles](../../test/e2e/misc/rbac_test.go) | 0 | ClusterRole/Role validation | RBAC roles have correct verbs and aggregate labels |
| [TestTelemetryLogs](../../test/e2e/misc/telemetry_log_analysis_test.go) | 4 (1 Trace + 1 Metric + 1 OTel Log + 1 FB Log) | All pipeline types; various inputs | No ERROR/WARNING in collector logs; no deprecation messages |
| [TestRejectTelemetryCRCreation](../../test/e2e/misc/telemetry_reject_creation_test.go) (5 subtests) | 0 | Invalid `collectionInterval` (zero/negative) at global, runtime, prometheus, istio | Webhook rejects Telemetry CR with invalid collection intervals |
| [TestTelemetryResources](../../test/e2e/misc/telemetry_resources_test.go) | 1 MetricPipeline | Default MetricPipeline | Self-monitor resources created on pipeline creation |
| [TestTelemetry](../../test/e2e/misc/telemetry_test.go) | 3 (1 each type) | Default OTLP output for all | Telemetry CR status has correct endpoints; webhooks configured; CA secret reconciled |
| [TestTelemetryWarning](../../test/e2e/misc/telemetry_test.go) | 1 TracePipeline | OTLP output from missing secret | Telemetry CR enters Warning state |
| [TestTelemetryDeletionBlocking](../../test/e2e/misc/telemetry_test.go) | 1 LogPipeline | Default LogPipeline | Deletion blocked by finalizer while pipeline exists; completes after pipeline deleted |

## Upgrade ([test/e2e/upgrade/](../../test/e2e/upgrade/))

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestLogsUpgrade](../../test/e2e/upgrade/logs_upgrade_test.go) | 2 (1 before + 1 after) | OTel OTLP HTTP output | OTel LogPipeline survives manager upgrade; new pipeline works post-upgrade |
| [TestLogsFluentBitUpgrade](../../test/e2e/upgrade/logs_fluentbit_upgrade_test.go) | 2 (1 before + 1 after) | FluentBit HTTP output; FIPS disabled | FluentBit LogPipeline survives upgrade; new pipeline works post-upgrade |
| [TestMetricsUpgrade](../../test/e2e/upgrade/metrics_upgrade_test.go) | 2 (1 before + 1 after) | OTLP HTTP output; default input | MetricPipeline survives upgrade; new pipeline works post-upgrade |
| [TestTracesUpgrade](../../test/e2e/upgrade/traces_upgrade_test.go) | 2 (1 before + 1 after) | OTLP HTTP output | TracePipeline survives upgrade; new pipeline works post-upgrade |
| [TestStorageMigration](../../test/e2e/upgrade/storage_migration_test.go) | 3 (1 each type) | All v1alpha1 API; upgrade from v1.55.0 | CRD storedVersions migrate from v1alpha1 to v1beta1 |

## Self-Monitor ([test/selfmonitor/](../../test/selfmonitor/))

These tests validate the self-monitoring stack: a Prometheus-based self-monitor that observes OTel Collector and Fluent Bit health metrics and surfaces pipeline conditions (FlowHealthy, etc.) based on alert rules.

### [TestHealthy](../../test/selfmonitor/healthy_test.go)

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestHealthy](../../test/selfmonitor/healthy_test.go) | 6 (one per component: log-agent, log-gateway, fluent-bit, metric-gateway, metric-agent, traces) | Each component gets its own pipeline with a healthy mock backend; 1 gateway replica; FIPS mode disabled for FluentBit | All 6 pipeline components reach healthy baseline; self-monitor Deployment is ready with active scrape targets; data is delivered to all backends; FlowHealthy condition remains stable for 3 minutes; correct self-monitor image (FIPS vs non-FIPS) is used |

### [TestBackpressure](../../test/selfmonitor/backpressure_test.go)

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestBackpressure/log-agent](../../test/selfmonitor/backpressure_test.go) | 1 | OTLP LogPipeline; fault backend with 30% non-retryable errors (HTTP 400) | Self-monitor detects `SomeDataDropped` for log agent |
| [TestBackpressure/log-gateway](../../test/selfmonitor/backpressure_test.go) | 1 | OTLP LogPipeline; fault backend with 30% non-retryable errors | Self-monitor detects `SomeDataDropped` for log gateway |
| [TestBackpressure/fluent-bit-buffer-filling-up](../../test/selfmonitor/backpressure_test.go) | 1 | HTTP LogPipeline; fault backend with 98% retryable errors (HTTP 429) + 3s delay on 200 responses; high log generation rate (60× default) | FluentBit retries fill the buffer faster than it drains → self-monitor fires `BufferFillingUp` alert |
| [TestBackpressure/fluent-bit-data-dropped](../../test/selfmonitor/backpressure_test.go) | 1 | HTTP LogPipeline; fault backend with 30% non-retryable errors (HTTP 400) | Self-monitor detects `SomeDataDropped` for FluentBit |
| [TestBackpressure/metric-gateway](../../test/selfmonitor/backpressure_test.go) | 1 | OTLP MetricPipeline; fault backend with 30% non-retryable errors | Self-monitor detects `SomeDataDropped` for metric gateway |
| [TestBackpressure/metric-agent](../../test/selfmonitor/backpressure_test.go) | 1 | OTLP MetricPipeline; Istio VirtualService fault injection (30% abort) targeting metric-agent pods only; high-load prometheus generator | Self-monitor detects `SomeDataDropped` for metric agent while gateway remains unaffected |
| [TestBackpressure/traces](../../test/selfmonitor/backpressure_test.go) | 1 | OTLP TracePipeline; fault backend with 30% non-retryable errors | Self-monitor detects `SomeDataDropped` for trace gateway |

### [TestOutage](../../test/selfmonitor/outage_test.go)

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestOutage/log-agent](../../test/selfmonitor/outage_test.go) | 1 | OTLP LogPipeline; fault backend with 100% non-retryable errors; faults active from boot | Self-monitor detects `AllDataDropped` for log agent |
| [TestOutage/log-gateway](../../test/selfmonitor/outage_test.go) | 1 | OTLP LogPipeline; fault backend with 100% non-retryable errors; faults active from boot | Self-monitor detects `AllDataDropped` for log gateway |
| [TestOutage/fluent-bit-no-logs-delivered](../../test/selfmonitor/outage_test.go) | 1 | HTTP LogPipeline; fault backend immediately closes TCP connections; faults active from boot | Self-monitor detects `NoLogsDelivered` for FluentBit (no HTTP response ever received) |
| [TestOutage/fluent-bit-all-data-dropped](../../test/selfmonitor/outage_test.go) | 1 | HTTP LogPipeline; fault backend with 100% non-retryable errors (HTTP 400) | Self-monitor detects `AllDataDropped` for FluentBit |
| [TestOutage/metric-gateway](../../test/selfmonitor/outage_test.go) | 1 | OTLP MetricPipeline; fault backend with 100% non-retryable errors; faults active from boot | Self-monitor detects `AllDataDropped` for metric gateway |
| [TestOutage/metric-agent](../../test/selfmonitor/outage_test.go) | 1 | OTLP MetricPipeline; Istio VirtualService fault injection (100% abort) targeting metric-agent pods; faults active from boot | Self-monitor detects `AllDataDropped` for metric agent while gateway remains unaffected |
| [TestOutage/traces](../../test/selfmonitor/outage_test.go) | 1 | OTLP TracePipeline; fault backend with 100% non-retryable errors; faults active from boot | Self-monitor detects `AllDataDropped` for trace gateway |

## Integration — Istio ([test/integration/istio/](../../test/integration/istio/))

These tests require an Istio-enabled cluster (Gardener label) and validate telemetry collection in service mesh environments.

| Test Name | # Pipelines | Configuration Aspects | Scenario Under Test |
|---|---|---|---|
| [TestAccessLogsOTLP](../../test/integration/istio/access_logs_otlp_test.go) (istio, istio-experimental) | 1 LogPipeline + 1 TracePipeline each | LogPipeline: OTLP output, runtime input disabled; TracePipeline: OTLP HTTP output; Istio-injected namespaces | Istio OTLP access logs delivered with correct attributes, severity (INFO), scope (`io.kyma-project.telemetry/istio`); cluster-level attributes absent; noise filter applied (no telemetry-otlp-traces spans) |
| [TestAccessLogsFluentBit](../../test/integration/istio/access_logs_test.go) | 1 LogPipeline | HTTP output; container selector (istio-proxy); FluentBit backend; FIPS disabled; Istio-injected namespace | Istio access logs collected from istio-proxy sidecar via FluentBit with expected attribute keys |
| [TestMetricsIstioInput](../../test/integration/istio/metrics_istio_input_test.go) | 1 MetricPipeline + 1 LogPipeline | MetricPipeline: Istio input enabled with namespace include selector, OTLP input disabled, OTLP HTTP output; LogPipeline: runtime input disabled, OTLP output; Istio-injected namespaces; traffic generators | Istio metrics (istio_requests_total, istio_request_duration_milliseconds, etc.) delivered with correct resource attributes (k8s.namespace, k8s.pod, service.name), metric attributes (connection_security_policy, destination_*, source_*), and scope; namespace filtering works; no diagnostic metrics leak; noise filtering (no telemetry-log-gateway destination) |
| [TestMetricsIstioInputEnvoy](../../test/integration/istio/metrics_istio_input_envoy_test.go) | 1 MetricPipeline | Istio input with envoy metrics enabled; namespace include selector; OTLP HTTP output; Istio-injected namespace; traffic generator | Envoy-specific metrics (envoy_cluster_version, envoy_cluster_upstream_rq_total, envoy_cluster_upstream_cx_total) are collected and delivered |
| [TestMetricsEnvoyMultiPipeline](../../test/integration/istio/metrics_envoy_multi_pipeline_test.go) | 2 MetricPipelines | Pipeline 1: Istio input with envoy metrics enabled + namespace include; Pipeline 2: Istio input with envoy metrics disabled + namespace exclude; different backends; 2 Istio-injected app namespaces | Envoy metrics delivered only to the pipeline with envoy metrics enabled; the other pipeline does not receive envoy metrics |
| [TestMetricsIstioSamePort](../../test/integration/istio/metrics_istio_same_port_test.go) | 2 MetricPipelines | Both: Prometheus input enabled; OTLP HTTP output; one backend plain, one with Istio injection; generators use same ports as backends; Prometheus HTTP and HTTPS annotations | Prometheus scraping works both inside and outside the Istio mesh when generators share ports with backends; metrics from both namespaces delivered to both backends |
| [TestMetricsOTLPInput](../../test/integration/istio/metrics_otlp_input_test.go) | 2 MetricPipelines | Both: default OTLP input; OTLP HTTP output; one backend plain, one Istio-injected with PeerAuthentication; telemetrygen metric producers in both namespaces | OTLP metric delivery works across Istio mesh boundary (plain → mesh, mesh → plain); metrics from both namespaces delivered to both backends |
| [TestMetricsPrometheusInput](../../test/integration/istio/metrics_prometheus_input_test.go) | 1 MetricPipeline | Prometheus input with namespace include selector; OTLP HTTP output; 3 metric producers: HTTPS-annotated with sidecar, HTTP-annotated without sidecar, unannotated | Prometheus scraping with Istio: pod-level scraping blocked for HTTPS-annotated sidecar pods (strict mTLS); pod scraping works for HTTP and unannotated pods; service-level scraping works for all three (including through mesh); correct instrumentation scope |
| [TestTracesRouting](../../test/integration/istio/traces_routing_test.go) | 2 TracePipelines | Both: OTLP HTTP output; one backend plain, one Istio-injected; apps inside and outside mesh; external trace gateway service | Traces routed correctly to both backends; Istio proxy spans present for mesh namespace; custom app spans delivered for all apps to both backends; noisy Istio spans (telemetry-otlp-traces calls) filtered out |

## Summary by Scenario Category

| Category | Traces | Logs | Metrics | Misc | Upgrade | Self-Monitor | Istio Integration |
|---|---|---|---|---|---|---|---|
| **Basic delivery** (single pipeline, protocols) | 2 | 8 | 4 | — | — | — | — |
| **v1alpha1 backward compatibility** | 1 | 2 | 2 | — | — | — | — |
| **Enrichment** (service name, cloud, cluster, pod labels) | 4 | 6 | 8 | — | — | — | — |
| **mTLS** (valid, invalid, expired, about-to-expire) | 6 | 10 | 7 | — | — | — | — |
| **OAuth2** | 1 | 3 | 2 | — | — | — | — |
| **Secret management** (missing, rotation) | 2 | 6 | 4 | — | — | — | — |
| **Multi-pipeline** (fanout, broken, max limit) | 3 | 9 | 6 | — | — | — | 1 |
| **Filters and Transforms** | 4 | 6 | 5 | — | — | — | — |
| **Validation / Rejection** (invalid configs) | 2 | 2 | 2 | 1 | — | — | — |
| **Namespace/Container selectors** | — | 4 | 2 | — | — | — | — |
| **Resource lifecycle** | 1 | 2 | 1 | 1 | — | — | — |
| **Endpoint validation** | 2 | 2 | 2 | — | — | — | — |
| **FluentBit-specific** (dedot, custom filter/output, app_name, keep) | — | 9 | — | — | — | — | — |
| **Agent-specific** (instrumentation scope, severity, trace parser) | — | 3 | — | — | — | — | — |
| **Input configuration** (disabled, prometheus, runtime, kyma) | — | 2 | 4 | — | — | — | — |
| **Noisy span/log filtering** | 1 | — | — | — | — | — | 2 |
| **Telemetry CR lifecycle** | — | — | — | 5 | — | — | — |
| **RBAC** | — | — | — | 2 | — | — | — |
| **Overrides** | — | — | — | 1 | — | — | — |
| **Custom labels/annotations** | — | — | — | 4 | — | — | — |
| **Upgrade and migration** | — | — | — | — | 5 | — | — |
| **Log analysis** (no errors in collector logs) | — | — | — | 1 | — | — | — |
| **Self-monitor: healthy baseline** | — | — | — | — | — | 1 (6 components) | — |
| **Self-monitor: backpressure** | — | — | — | — | — | 7 | — |
| **Self-monitor: outage** | — | — | — | — | — | 7 | — |
| **Istio: access logs** | — | — | — | — | — | — | 2 |
| **Istio: metrics** (istio input, envoy, prometheus, OTLP) | — | — | — | — | — | — | 5 |
| **Istio: trace routing** | — | — | — | — | — | — | 1 |
