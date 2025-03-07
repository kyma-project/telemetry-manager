# OTel Client Retries

The PoC finds out the behaviour of OTel Collector clients (instrumented apps, istio proxies) in case of the collector downtime (non-ha setup).

## OTel SDK

Most of the clients shipping data to a collector will be apps instrumented with otel sdk. Otel sdk specification ist ziemlich konrett about retries:
https://opentelemetry.io/docs/specs/otel/trace/sdk/#exportbatch
https://opentelemetry.io/docs/specs/otel/metrics/sdk/#exportbatch
https://opentelemetry.io/docs/specs/otel/logs/sdk/#export

For example, for traces
> Concurrent requests and retry logic is the responsibility of the exporter. The default SDK’s Span Processors SHOULD NOT implement retry logic, as the required logic is likely to depend heavily on the specific protocol and backend the spans are being sent to. For example, the OpenTelemetry Protocol (OTLP) specification defines logic for both sending concurrent requests and retrying requests

How to test it?

```bash
# From root repo dir

# Provision a k3d cluster with istio
make provision-k3d 
./hack/install-istio.sh

# Deploy telemetry manager
make deploy

# Deploy telemetrygen, a mock in-cluster trace backend, and a TracePipeline that connects them
k apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/refs/heads/main/docs/contributor/pocs/otel-client-retries/telemetry_v1alpha1_tracepipeline_otlphttp.yaml

# Simulate permanent non-retriable errors
k apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/refs/heads/main/docs/contributor/pocs/otel-client-retries/vs-trace-gateway-fault-404.yaml

```

Looks at the logs of the telemetrygen pod, you will it spamming the following errors:
```bash
traces export: failed to send to http://telemetry-otlp-traces.kyma-system:4318/v1/traces: 404 Not Found (body: fault filter abort)
```

```bash
# Simulate retriable errors
k apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/refs/heads/main/docs/contributor/pocs/otel-client-retries/vs-trace-gateway-fault-429.yaml

```

