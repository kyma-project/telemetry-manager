# Configure OpenTelemetry Zero-Code Instrumentation

To collect traces from applications without modifying their code, use the [OpenTelemetry Operator](https://opentelemetry.io/docs/kubernetes/operator/) auto-instrumentation feature. The operator injects an instrumentation agent into your workloads at pod creation time and exports traces to the Kyma Telemetry module's OTLP endpoint.

The Kyma Telemetry module manages its own OTel Collectors but does not ship the OTel Operator. Install the operator independently and point its `Instrumentation` CR at the module's OTLP endpoint. Both components run side by side without competing for the same resources.

## Trade-Offs

Before enabling auto-instrumentation, consider the following:

| Benefit                                  | Limitation                                                                                                                               |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| No application code changes required     | Injected code runs inside your application process, a bug in the agent can crash your application                                        |
| Works for applications you don't control | Injected code has access to your application's memory and secrets                                                                        |
| Covers multiple languages with one CR    | Traces are less detailed than SDK-based instrumentation, depending on the language                                                       |
| Enables tracing for legacy applications  | Changes to the `Instrumentation` CR only take effect after a pod restart                                                                 |
|                                          | Go instrumentation requires a privileged sidecar (`runAsUser: 0`), which is incompatible with `Restricted` PodSecurityAdmission policies |
|                                          | `Instrumentation` CR uses the `v1alpha1` API, which may change without a deprecation period                                              |

## Prerequisites

- Kyma cluster with the **Telemetry** and **Istio** modules enabled
- **cert-manager** installed (required by the operator's admission webhook):

  ```bash
  helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
    --namespace cert-manager --create-namespace \
    --version v1.21.2 \
    --values k8s/cert-manager-values.yaml
  ```

## Steps

### 1. Install the OTel Operator

```bash
helm repo add opentelemetry-helm https://open-telemetry.github.io/opentelemetry-helm-charts
helm install my-opentelemetry-operator opentelemetry-helm/opentelemetry-operator \
  --namespace opentelemetry-operator-system --create-namespace \
  --version 0.123.1 \
  --values k8s/otel-operator-values.yaml
```

The operator installs its CRDs (`Instrumentation`, `OpenTelemetryCollector`) and a mutating webhook that injects agents at pod creation time. The values file enables Go auto-instrumentation (`autoInstrumentation.go.enabled: true`) and wires the webhook to cert-manager; both are off by default and must be set explicitly.

### 2. Create an Instrumentation CR

The `Instrumentation` CR tells the operator which OTLP endpoint to export to, which propagators to use, and how to sample. Create one per namespace.

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
> The `Instrumentation` CR exports using HTTP/protobuf only, so always use port `4318`. Check the [OTel auto-instrumentation docs](https://opentelemetry.io/docs/kubernetes/operator/automatic/) for any language-specific endpoint requirements.

### 3. Annotate Workloads

The operator watches for pod annotations and injects the appropriate agent at pod creation time. Add the annotation to the pod template of your `Deployment`:

```yaml
# Java
instrumentation.opentelemetry.io/inject-java: "my-instrumentation"

# Node.js
instrumentation.opentelemetry.io/inject-nodejs: "my-instrumentation"

# Go (eBPF sidecar, see Go section below)
instrumentation.opentelemetry.io/inject-go: "my-instrumentation"
```

The value is the name of the `Instrumentation` CR in the same namespace.

To inject into every new pod in a namespace, add the annotation to the namespace itself:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  annotations:
    instrumentation.opentelemetry.io/inject-java: "my-instrumentation"
```

> [!NOTE]
> Namespace-level injection works for Java, Node.js, Python, and .NET. For Go, you must still set the `instrumentation.opentelemetry.io/otel-go-instrumentation-container` annotation on each pod, so namespace-level injection alone is insufficient.

## Sampler Configuration and Istio

When Istio is active in your cluster, its Envoy proxies propagate the W3C `traceparent` header, which carries the sampling decision. Because the OTel agent uses `parentbased_traceidratio`, it honors the sampling flag set by Istio, if Istio decided not to sample a request, the agent does not report a span for it either.

To control trace volume entirely through Istio, set `randomSamplingPercentage` in your Istio `Telemetry` CR and set the instrumentation sampler argument to `"0"`:

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

Setting the argument to `"0"` means the agent never starts a new trace on its own. It only generates spans when an incoming `traceparent` header is already marked as sampled by Istio. Internal traffic such as health checks (`/healthz`, `/readyz`) that enters without a sampled `traceparent` produces no spans.

> [!WARNING]
> If you set the argument to `"1"` instead, the agent generates spans for every request it handles as a root span, including internal endpoints. Those spans are collected regardless of Istio's sampling decision.

For full details on Istio tracing configuration, see [Configure Istio Tracing](./istio-support.md).

## Resource Limits

Rewriting bytecode (Java) or attaching eBPF probes (Go) adds CPU and memory overhead. Tight resource limits cause slowdowns.

Set limits in the `Instrumentation` CR under the runtime key. The right values depend on your workload. For guidance on sizing, see the [OTel Operator resource documentation](https://opentelemetry.io/docs/kubernetes/operator/automatic/).

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

The Java agent enables metrics and logs exporters by default. Disable them because the `telemetry-otlp-traces` endpoint only accepts `/v1/traces`:

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

Annotate the pod template of your `Deployment`:

```yaml
spec:
  template:
    metadata:
      annotations:
        instrumentation.opentelemetry.io/inject-java: "my-instrumentation-java"
```

> [!NOTE]
> The Java agent modifies bytecode at startup using an init container. Expect additional startup time. Tune resource limits according to the [OTel Java agent docs](https://opentelemetry.io/docs/zero-code/java/agent/).

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

Annotate the pod template of your `Deployment`:

```yaml
spec:
  template:
    metadata:
      annotations:
        instrumentation.opentelemetry.io/inject-nodejs: "my-instrumentation-nodejs"
```

### Go (eBPF Sidecar)

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

> [!WARNING]
> `runAsUser: 0` is incompatible with a `Restricted` PodSecurityAdmission policy. You must grant a `Privileged` or `Baseline` exemption to the namespace before deploying. In many production environments this is not acceptable. Evaluate this requirement before using Go auto-instrumentation.

## Known Limitations

- **`v1alpha1` API stability.** The `Instrumentation` CR is `v1alpha1`. The spec may change in future operator releases without a deprecation period.
- **Pod restart required.** Changes to the `Instrumentation` CR; such as endpoint, agent version, or environment variables — are not applied to running pods. Restart the deployment to pick up changes.
- **Trace detail varies by language.** Auto-instrumentation covers common frameworks and libraries but does not instrument custom code. SDK-based instrumentation provides more granular spans.
- **Agent instability risk.** The injected agent runs inside your application process. A bug in the agent can cause your application to crash or behave unexpectedly.
- **Security exposure.** The injected code has access to your application's memory space and environment variables, including secrets. For Go, the eBPF sidecar runs as root and has broader system access. Evaluate the security implications before using auto-instrumentation in production.
- **Go eBPF requires root.** The Go sidecar must run as `runAsUser: 0`. This is a hard requirement of the eBPF subsystem.
