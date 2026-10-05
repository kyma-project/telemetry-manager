# Configure OpenTelemetry Auto-Instrumentation

To collect traces from applications without modifying their code, use the [OpenTelemetry (OTel) Operator](https://opentelemetry.io/docs/kubernetes/operator/) auto-instrumentation feature. The operator injects an instrumentation agent into your workloads at Pod creation time and exports traces to the Kyma Telemetry module's OTLP endpoint.

The Kyma Telemetry module manages its own OTel Collectors but does not ship the OTel Operator. Install the operator independently and point its Instrumentation CR at the module's OTLP endpoint. Both components run side by side without competing for the same resources.

## Benefits and Limitations

Before you enable auto-instrumentation, consider the following benefits and limitations:

| Benefit                                  | Limitation                                                                     |
| ---------------------------------------- | ------------------------------------------------------------------------------ |
| No application code changes required     | Injected code runs inside your application process and can affect its behavior |
| Works for applications you don't control | Injected code has access to your application's memory and secrets              |
| Covers multiple languages with one CR    | Traces are less detailed than code-based instrumentation                       |
| Enables tracing for legacy applications  | Changes take effect only after a Pod restart                                   |

For the full set of constraints, see [Known Limitations](#known-limitations).

## Prerequisites

- Kyma cluster with the **Telemetry** and **Istio** modules enabled
- **cert-manager** installed, which the operator uses by default to provision its admission webhook certificates. Install only the CRDs and controller:

  ```bash
  helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
    --namespace cert-manager --create-namespace \
    --version v1.21.2 \
    --set crds.enabled=true
  ```

## Steps

### 1. Install the OTel Operator

```bash
helm repo add opentelemetry-helm https://open-telemetry.github.io/opentelemetry-helm-charts
helm install my-opentelemetry-operator opentelemetry-helm/opentelemetry-operator \
  --namespace opentelemetry-operator-system --create-namespace \
  --version 0.123.1 \
  --set manager.autoInstrumentation.go.enabled=true
```

The operator installs its CRDs, Instrumentation and OpenTelemetryCollector, and a mutating webhook that injects agents at Pod creation time. Go auto-instrumentation is off by default, so you must enable it explicitly with `manager.autoInstrumentation.go.enabled=true`. For other languages, you can omit this flag.

### 2. Create an Instrumentation CR

The Instrumentation CR tells the operator which OTLP endpoint to export to, which propagators to use, and how to sample. Create one per namespace.

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: my-instrumentation
  namespace: my-app
spec:
  exporter:
    endpoint: http://telemetry-otlp-traces.kyma-system.svc.cluster.local:4318
  propagators:
    - tracecontext
    - baggage
  sampler:
    type: parentbased_traceidratio
    argument: "1"
```

> [!NOTE]
> The Instrumentation CR exports using HTTP/protobuf only, so always use port `4318`. Check the [OTel auto-instrumentation docs](https://opentelemetry.io/docs/kubernetes/operator/automatic/) for any language-specific endpoint requirements.

### 3. Annotate Workloads

The operator watches for Pod annotations and injects the appropriate agent at Pod creation time. Add the annotation to the Pod template of your Deployment:

```yaml
# Java
instrumentation.opentelemetry.io/inject-java: "my-instrumentation"

# Node.js
instrumentation.opentelemetry.io/inject-nodejs: "my-instrumentation"

# Go, which uses the eBPF sidecar described in the Go section
instrumentation.opentelemetry.io/inject-go: "my-instrumentation"
```

The value is the name of the Instrumentation CR in the same namespace.

To inject into every new Pod in a namespace, add the annotation to the namespace itself:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  annotations:
    instrumentation.opentelemetry.io/inject-java: "my-instrumentation"
```

Namespace-level injection works for Java, Node.js, Python, and .NET. For Go, you must still set the `instrumentation.opentelemetry.io/otel-go-instrumentation-container` annotation on each Pod, so namespace-level injection alone is insufficient.

## Sampler Configuration and Istio

When Istio is active in your cluster, its Envoy proxies propagate the W3C `traceparent` header, which carries the sampling decision. Because the OTel agent uses `parentbased_traceidratio`, it honors the sampling flag set by Istio. If Istio decides not to sample a request, the agent does not report a span for it either.

To control trace volume entirely through Istio, set `randomSamplingPercentage` in your Istio Telemetry CR and set the instrumentation sampler argument to `"0"`:

```yaml
# Istio Telemetry CR: controls which traces enter the mesh
spec:
  tracing:
    - providers:
        - name: "kyma-traces"
      randomSamplingPercentage: 1.00 # 1% sampling
```

```yaml
# Instrumentation CR: only adds app spans when Istio already decided to sample
spec:
  sampler:
    type: parentbased_traceidratio
    argument: "0"
```

The agent always honors the sampling decision in an incoming `traceparent` header. The argument applies only to requests that arrive without a parent, such as a kubelet health probe that reaches the Pod directly instead of through Istio. With `"0"`, the agent starts no trace for such requests. With `"1"`, the agent starts a root span for each one, so health checks produce spans that Istio never saw.

For full details on Istio tracing configuration, see [Configure Istio Tracing](../../collecting-traces/istio-support.md).

## Resource Limits

Rewriting bytecode for Java or attaching eBPF probes for Go adds CPU and memory overhead. Tight resource limits cause slowdowns.

Set limits in the Instrumentation CR under the runtime key. The right values depend on your workload. For guidance on sizing, see the [OTel Operator resource documentation](https://opentelemetry.io/docs/kubernetes/operator/automatic/).

```yaml
spec:
  java:
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
  nodejs:
    resources:
      limits:
        cpu: 200m
        memory: 128Mi
```

## Language-Specific Examples

### Java

The Java agent enables metrics and logs exporters by default. If you want to collect only traces, disable them, because the `telemetry-otlp-traces` endpoint accepts only `/v1/traces`:

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: my-instrumentation-java
  namespace: my-app
spec:
  exporter:
    endpoint: http://telemetry-otlp-traces.kyma-system.svc.cluster.local:4318
  propagators:
    - tracecontext
    - baggage
  sampler:
    type: parentbased_traceidratio
    argument: "1"
  java:
    env:
      - name: OTEL_METRICS_EXPORTER
        value: none
      - name: OTEL_LOGS_EXPORTER
        value: none
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
```

Annotate the Pod template of your Deployment:

```yaml
spec:
  template:
    metadata:
      annotations:
        instrumentation.opentelemetry.io/inject-java: "my-instrumentation-java"
```

The Java agent modifies bytecode at startup using an init container. Expect additional startup time. Tune resource limits according to the [OTel Java agent docs](https://opentelemetry.io/docs/zero-code/java/agent/).

### Node.js

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: my-instrumentation-nodejs
  namespace: my-app
spec:
  exporter:
    endpoint: http://telemetry-otlp-traces.kyma-system.svc.cluster.local:4318
  propagators:
    - tracecontext
    - baggage
  sampler:
    type: parentbased_traceidratio
    argument: "1"
  nodejs:
    resources:
      limits:
        cpu: 200m
        memory: 128Mi
```

Annotate the Pod template of your Deployment:

```yaml
spec:
  template:
    metadata:
      annotations:
        instrumentation.opentelemetry.io/inject-nodejs: "my-instrumentation-nodejs"
```

### Go eBPF Sidecar

Go auto-instrumentation uses an eBPF sidecar rather than an init container. The sidecar must run as root to access eBPF maps:

```yaml
annotations:
  instrumentation.opentelemetry.io/inject-go: "my-instrumentation"
  instrumentation.opentelemetry.io/otel-go-instrumentation-container: "my-container-name"
spec:
  containers:
    - name: my-container-name
      securityContext:
        runAsUser: 0
```

> [!CAUTION]
> `runAsUser: 0` conflicts with a `Restricted` PodSecurityAdmission policy. You must grant the namespace a `Privileged` or `Baseline` exemption before you deploy. Many production environments do not accept this. Evaluate this requirement before you use Go auto-instrumentation.

## Known Limitations

- **`v1alpha1` API stability.** The Instrumentation CR uses the `v1alpha1` API. The spec might change in future operator releases without a deprecation period.
- **Pod restart required.** Changes to the Instrumentation CR, such as the endpoint, agent version, or environment variables, do not reach running Pods. Restart the Deployment to pick up changes.
- **Trace detail varies by language.** Auto-instrumentation covers common frameworks and libraries but does not instrument custom code. Code-based instrumentation provides more granular spans.
- **Agent instability risk.** The injected agent runs inside your application process. A bug in the agent can crash your application or make it behave unexpectedly.
- **Security exposure.** The injected code has access to your application's memory space and environment variables, including secrets. For Go, the eBPF sidecar runs as root and has broader system access. Evaluate the security implications before you use auto-instrumentation in production.
- **Go eBPF requires root.** The Go sidecar must run as `runAsUser: 0`. This is a hard requirement of the eBPF subsystem.
