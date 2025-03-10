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

### 1. Setup Environment

Run the following commands from the root directory of the repository to provision the necessary environment:

```bash
# Provision a k3d cluster with Istio
make provision-k3d
./hack/install-istio.sh

# Deploy the telemetry manager
make deploy
```

### 2. Deploy Test Workloads

Deploy `telemetrygen`, a mock in-cluster trace backend, and a `TracePipeline` that connects them:

```bash
k apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/refs/heads/main/docs/contributor/pocs/otelcol-downtime/telemetry_v1alpha1_tracepipeline_otlphttp.yaml
```

### 3. Simulate Permanent Non-Retriable Errors

```bash
k apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/refs/heads/main/docs/contributor/pocs/otelcol-downtime/vs-trace-gateway-fault-404.yaml
```

Check the logs of the `telemetrygen` pod, where you will see it continuously logging the following errors:

```bash
traces export: failed to send to http://telemetry-otlp-traces.kyma-system:4318/v1/traces: 404 Not Found (body: fault filter abort)
```

### 4. Simulate Retriable Errors

```bash
k apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/refs/heads/main/docs/contributor/pocs/otelcol-downtime/vs-trace-gateway-fault-429.yaml
```

You will observe errors appearing every 30 seconds:

```bash
traces export: context deadline exceeded: retry-able request failure: body: fault filter abort
```

This confirms that retries are occurring as expected.
