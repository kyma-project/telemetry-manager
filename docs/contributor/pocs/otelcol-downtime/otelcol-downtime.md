# OpenTelemetry Collector Downtime PoC

This Proof of Concept (PoC) explores the behavior of OpenTelemetry (OTel) Collector clients — such as instrumented applications and Istio proxies — during collector downtime in a non-HA (High Availability) setup.

## OpenTelemetry SDK Behavior

Most clients sending data to an OTel Collector are applications instrumented with the OTel SDK. The OTel SDK specification clearly defines retry behavior:

- [Trace SDK Export](https://opentelemetry.io/docs/specs/otel/trace/sdk/#exportbatch)
- [Metrics SDK Export](https://opentelemetry.io/docs/specs/otel/metrics/sdk/#exportbatch)
- [Logs SDK Export](https://opentelemetry.io/docs/specs/otel/logs/sdk/#export)

For example, regarding trace exports:
> Concurrent requests and retry logic are the responsibility of the exporter. The default SDK’s Span Processors SHOULD NOT implement retry logic, as the required logic is likely to depend heavily on the specific protocol and backend the spans are being sent to. For example, the OpenTelemetry Protocol (OTLP) specification defines logic for both sending concurrent requests and retrying requests.


## How to Test

### 1. Set Up Environment

To provision the environment, run the following command from the root directory of the repository:

```bash
# Provision a k3d cluster with Istio
make provision-k3d
```

### 2. OTLP gRPC Testing

To simulate downtime and record logs, deploy `telemetrygen`, instrumented with the OTLP gRPC exporter, along with a service that has no backing Pods:

```bash
kubectl apply -f ./telemetrygen_otlpgrpc.yaml
```

Check the `telemetrygen` logs, where you should see the following message:

```bash
2025/03/11 10:36:43 traces export: context deadline exceeded: rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 10.43.93.51:4317: connect: connection refused"
```

According to the [OTLP specification](https://opentelemetry.io/docs/specs/otlp/), the `Unavailable` gRPC error code is considered retryable.

To clean up, delete the deployment:

```bash
kubectl delete -f ./telemetrygen_otlpgrpc.yaml
```

### 3. OTLP HTTP Testing

To simulate downtime and record logs, deploy `telemetrygen`, instrumented with the OTLP HTTP exporter, along with a service that has no backing Pods:

```bash
kubectl apply -f ./telemetrygen_otlphttp.yaml
```

Check the `telemetrygen` logs, where you should see the following message:

```bash
2025/03/11 10:42:44 traces export: Post "http://telemetry-otlp-traces.kyma-system:4318/v1/traces": dial tcp 10.43.48.18:4318: connect: connection refused
```

Unlike gRPC, these messages are logged every second, indicating that no retry mechanism is in place and data is being dropped.

To clean up, delete the deployment:

```bash
kubectl delete -f ./telemetrygen_otlphttp.yaml
```

### 4. OTLP gRPC with Istio

Install Istio:

```bash
./hacks/deploy-istio.sh
```

The results are similar to **OTLP gRPC Testing**, but with a different log message:

```bash
2025/03/11 10:53:38 traces export: context deadline exceeded: rpc error: code = Unavailable desc = no healthy upstream
```

### 5. OTLP HTTP with Istio

Install Istio:

```bash
./hacks/deploy-istio.sh
```

Unlike in **OTLP HTTP Testing**, retry behavior is observed, as evidenced by log messages appearing at a significantly lower rate (approximately 1–2 times per minute):

```bash
2025/03/11 10:57:40 traces export: context deadline exceeded: retry-able request failure: body: no healthy upstream
```

## Summary

- **OTLP gRPC without Istio**: Retries occur as expected, following the OTLP specification, when encountering an `Unavailable` error.
- **OTLP HTTP without Istio**: No retry mechanism is observed; data loss occurs when the collector is unavailable.
- **OTLP gRPC with Istio**: Similar behavior to gRPC without Istio
- **OTLP HTTP with Istio**: Unlike standard OTLP HTTP behavior, retries occur with Istio

## Istio Proxies  

Istio proxies can send access logs and spans to an OTLP endpoint, but they do not appear to use the OpenTelemetry (OTel) SDK. However, Envoy provides a way to configure retry policies:  

- [Envoy OpenTelemetry Trace Configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/trace/v3/opentelemetry.proto.html)  
- [Envoy OpenTelemetry Access Logger Configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/open_telemetry/v3/logs_service.proto)  

These configurations are not reflected in Istio's mesh configuration by default. However, enabling them is relatively straightforward. A feature request similar to [this one](https://github.com/istio/istio/issues/52873) could be submitted to improve support for retry policies.  

Additionally, tests indicate that Istio proxies currently do not implement any retry functionality. As a result, if the collector is unavailable, data is dropped.  
