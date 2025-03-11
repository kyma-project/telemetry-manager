# OpenTelemetry Collector Downtime PoC

This Proof of Concept (PoC) explores the behavior of OpenTelemetry (OTel) Collector clients—such as instrumented applications and Istio proxies—during collector downtime in a non-HA (High Availability) setup.

## OpenTelemetry SDK Behavior

Most clients sending data to an OTel Collector are applications instrumented with the OTel SDK. The OTel SDK specification clearly defines retry behavior:

- [Trace SDK Export](https://opentelemetry.io/docs/specs/otel/trace/sdk/#exportbatch)
- [Metrics SDK Export](https://opentelemetry.io/docs/specs/otel/metrics/sdk/#exportbatch)
- [Logs SDK Export](https://opentelemetry.io/docs/specs/otel/logs/sdk/#export)

For example, regarding trace exports:
> Concurrent requests and retry logic are the responsibility of the exporter. The default SDK’s Span Processors SHOULD NOT implement retry logic, as the required logic is likely to depend heavily on the specific protocol and backend the spans are being sent to. For example, the OpenTelemetry Protocol (OTLP) specification defines logic for both sending concurrent requests and retrying requests.

## How to Test

### 1. Set Up Environment

Run the following command from the root directory of the repository to provision the necessary environment:

```bash
# Provision a k3d cluster with Istio
make provision-k3d
```

### 2. OTLP gRPC Testing

Deploy `telemetrygen`, instrumented with the OTLP gRPC exporter, along with a service that has no backing Pods to simulate downtime and record logs:

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

Deploy `telemetrygen`, instrumented with the OTLP HTTP exporter, along with a service that has no backing Pods to simulate downtime and record logs:

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
