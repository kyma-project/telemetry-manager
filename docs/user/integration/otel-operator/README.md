# Configure OpenTelemetry Auto-Instrumentation

## Overview

| Category     |                                       |
| ------------ | ------------------------------------- |
| Signal types | traces                                |
| Backend type | custom in-cluster, third-party remote |
| OTLP-native  | yes                                   |

Learn how to use the [OpenTelemetry (OTel) Operator](https://opentelemetry.io/docs/kubernetes/operator/) to collect traces from your Kubernetes workloads without modifying application code. The operator injects an instrumentation agent into your Pods at creation time and exports traces to the Kyma Telemetry module's OTLP endpoint. Install the operator independently and point its Instrumentation CR at the module's OTLP endpoint. Both components run side by side without competing for the same resources.

## Table of Contents

- [Benefits and Limitations](#benefits-and-limitations)
- [Prerequisites](#prerequisites)
- [Install the OTel Operator](#install-the-otel-operator)
- [Create an Instrumentation CR](#create-an-instrumentation-cr)
- [Annotate Workloads](#annotate-workloads)
- [Sampler Configuration and Istio](#sampler-configuration-and-istio)
- [Resource Limits](#resource-limits)
- [Language-Specific Examples](#language-specific-examples)
- [Verify the Installation](#verify-the-installation)
- [Clean Up](#clean-up)

## Benefits and Limitations

Auto-instrumentation has trade-offs worth considering before you enable it.

Consider the following benefits:

- You don't need to modify your application code.
- It works for applications you don't own or control.
- One Instrumentation CR covers multiple languages.
- You can add tracing to legacy applications.

However, the following limitations apply:

- The injected agent runs inside your application process and can affect its stability.
- The injected code has access to your application's memory and environment variables, including secrets.
- Auto-instrumentation produces less detailed traces than code-based instrumentation.
- Changes to the Instrumentation CR take effect only after a Pod restart.
- The Instrumentation CR uses the `v1alpha1` API, which can change without a deprecation period.
- Go instrumentation requires the eBPF sidecar to run as root, which is incompatible with `Restricted` PodSecurityAdmission policies.

## Prerequisites

- Kyma cluster with the **Telemetry** and **Istio** modules enabled
- **cert-manager** installed, which the operator uses by default to provision its admission webhook certificates. Install only the CRDs and controller:

  ```bash
  helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
    --namespace cert-manager --create-namespace \
    --version v1.21.2 \
    --set crds.enabled=true
  ```

## Install the OTel Operator

1. Add the OTel Helm repository:

   ```bash
   helm repo add opentelemetry-helm https://open-telemetry.github.io/opentelemetry-helm-charts
   ```

1. Install the operator:

   ```bash
   helm install my-opentelemetry-operator opentelemetry-helm/opentelemetry-operator \
     --namespace opentelemetry-operator-system --create-namespace \
     --version 0.123.1 \
     --set manager.autoInstrumentation.go.enabled=true
   ```

The operator installs its CRDs, Instrumentation and OpenTelemetryCollector, and a mutating webhook that injects agents at Pod creation time. Go auto-instrumentation is off by default, so you must enable it explicitly with `manager.autoInstrumentation.go.enabled=true`. For other languages, you can omit this flag.

## Create an Instrumentation CR

The Instrumentation CR tells the operator which OTLP endpoint to export to, which propagators to use, and how to sample. By default, the operator looks for the CR in the same namespace as the workload. To reuse a single CR across namespaces, reference it using `"{NAMESPACE}/{NAME}"` in the annotation.

1. Create an Instrumentation CR:

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

## Annotate Workloads

The operator watches for Pod annotations and injects the appropriate agent at Pod creation time.

1. Add the annotation to the Pod template of your Deployment:

   ```yaml
   # Java
   instrumentation.opentelemetry.io/inject-java: "my-instrumentation"

   # Node.js
   instrumentation.opentelemetry.io/inject-nodejs: "my-instrumentation"

   # Go, which uses the eBPF sidecar described in the Go section
   instrumentation.opentelemetry.io/inject-go: "my-instrumentation"
   ```

   The value is the name of the Instrumentation CR in the same namespace.

1. To inject into every new Pod in a namespace, add the annotation to the namespace itself:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: my-app
     annotations:
       instrumentation.opentelemetry.io/inject-java: "my-instrumentation"
   ```

   Namespace-level injection works for Java, Node.js, Python, and .NET. For Go, you must still set the `instrumentation.opentelemetry.io/otel-go-instrumentation-container` annotation on each Pod, so namespace-level injection alone is insufficient.

## Configure Sampling with Istio

When Istio is active in your cluster, its Envoy proxies propagate the W3C `traceparent` header, which carries the sampling decision. Because the OTel agent uses `parentbased_traceidratio`, it honors the sampling flag set by Istio. If Istio decides not to sample a request, the agent does not report a span for it either.

To control trace volume entirely through Istio, set `randomSamplingPercentage` in your Istio Telemetry CR and set the instrumentation sampler argument to `"0"`:

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
  namespace: istio-system
spec:
  tracing:
    - providers:
        - name: "kyma-traces"
      randomSamplingPercentage: 1.00 # adjust this value
```

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: my-instrumentation
  namespace: my-app
spec:
  sampler:
    type: parentbased_traceidratio
    argument: "0" # only adds app spans when Istio already decided to sample
```

The agent always honors the sampling decision in an incoming `traceparent` header. The argument applies only to requests that arrive without a parent, such as a kubelet health probe that reaches the Pod directly instead of through Istio. With `"0"`, the agent starts no trace for such requests. With `"1"`, the agent starts a root span for each one, so health checks produce spans that Istio never saw.

For full details on Istio tracing configuration, see [Configure Istio Tracing](../../collecting-traces/istio-support.md).

## Set Resource Limits

Rewriting bytecode for Java or attaching eBPF probes for Go adds CPU and memory overhead. If limits are too low, Pod startup slows significantly.

Set limits and requests in the Instrumentation CR under the language-specific key. The right values depend on your workload. For guidance on sizing, see the [OTel Operator resource documentation](https://opentelemetry.io/docs/kubernetes/operator/automatic/).

```yaml
spec:
  java:
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
  nodejs:
    resourceRequirements:
      limits:
        cpu: 200m
        memory: 128Mi
```

## Exporting Metrics and Logs

By default, the OpenTelemetry SDK attempts to export traces, metrics, and logs using OTLP. The `exporter.endpoint` used in this guide, `telemetry-otlp-traces.kyma-system.svc.cluster.local:4318`, accepts only traces. Therefore, the language-specific examples below explicitly configure the metrics and logs exporters to avoid sending them to the trace-only endpoint.

To collect metrics and logs, you must explicitly configure where to send them. You can use the following approaches.

### Route to Kyma Telemetry Pipelines

If you configured a `TracePipeline`, `MetricPipeline`, or `LogPipeline` using the Kyma Telemetry module, you can route the signals to their respective OTLP endpoints by overriding the environment variables in your `Instrumentation` CR.

For a complete list of standard OTLP endpoint environment variables, see the [OpenTelemetry SDK Environment Variables Specification](https://opentelemetry.io/docs/specs/otel/protocol/exporter/).

```yaml
spec:
  env:
    - name: OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
      value: "http://telemetry-otlp-metrics.kyma-system.svc.cluster.local:4318/v1/metrics"
    - name: OTEL_EXPORTER_OTLP_LOGS_ENDPOINT
      value: "http://telemetry-otlp-logs.kyma-system.svc.cluster.local:4318/v1/logs"
```

### Use Alternative Built-In Exporters

Alternatively, you can bypass OTLP for metrics and logs entirely. For example, you can expose metrics for Prometheus to scrape and disable logs to avoid duplicating your application's standard output. For debugging, you can set logs to `console` instead.

The availability of built-in exporters such as `prometheus` or `console` is language-specific. For example, Java and Node.js support them, but the Go eBPF auto-instrumentation currently does not. To confirm support, always check the official OpenTelemetry documentation for your specific language.

For the full list of supported built-in exporters, see the [Exporter Selection Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#exporter-selection).

```yaml
spec:
  env:
    - name: OTEL_METRICS_EXPORTER
      value: prometheus
    - name: OTEL_LOGS_EXPORTER
      value: none
```

Setting `prometheus` starts a local HTTP server on the Pod, typically on port `9464`, exposing a `/metrics` path. Setting `console` prints all OpenTelemetry logs to the container's `stdout`, which can cause duplicate logging if your application already logs to standard output.

## Language-Specific Examples

The OTel Operator controls which languages are supported for auto-instrumentation. The list might change as the operator evolves. This guide provides tested examples for Java, Node.js, and Go. For other supported languages such as Python and .NET, see the [OTel auto-instrumentation documentation](https://opentelemetry.io/docs/kubernetes/operator/automatic/).

### Java

The Java agent modifies bytecode at startup using an init container:

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
        value: prometheus
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

Expect additional startup time because the agent modifies bytecode before the application starts. Tune resource limits according to the [OTel Java agent docs](https://opentelemetry.io/docs/zero-code/java/agent/).

### Node.js

The Node.js agent attaches at process startup using an init container. It prepends the OTel SDK to the Node.js require chain, so no changes to your application code are needed:

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
    env:
      - name: OTEL_METRICS_EXPORTER
        value: prometheus
      - name: OTEL_LOGS_EXPORTER
        value: none
    resourceRequirements:
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

Go auto-instrumentation uses an eBPF sidecar rather than an init container. The sidecar attaches to the target process using eBPF probes at the kernel level, intercepting function calls without modifying the application binary. The sidecar must run as root to access eBPF maps:

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: my-instrumentation-go
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
  go:
    resourceRequirements:
      limits:
        cpu: 200m
        memory: 128Mi
```

Annotate the Pod template of your Deployment:

```yaml
metadata:
  annotations:
    instrumentation.opentelemetry.io/inject-go: "my-instrumentation-go"
    instrumentation.opentelemetry.io/otel-go-instrumentation-container: "my-container-name"
spec:
  containers:
    - name: my-container-name
      securityContext:
        runAsUser: 0
```

> [!WARNING]
> `runAsUser: 0` conflicts with a `Restricted` PodSecurityAdmission policy. You must grant the namespace a `Privileged` or `Baseline` exemption before you deploy. Many production environments do not accept this. Evaluate this requirement before you use Go auto-instrumentation.

## Verify the Installation

1. Check that the Instrumentation CR was created successfully:

   ```bash
   kubectl get instrumentation -n my-app
   ```

1. If you have an annotated workload, verify that the agent was injected into the Pod. For Java and Node.js, look for an init container; for Go, look for a sidecar container:

   ```bash
   kubectl describe pod --namespace {NAMESPACE} -l {WORKLOAD_LABEL}
   ```

   For Java and Node.js, look for `opentelemetry-auto-instrumentation` in the init containers. For Go, look for `opentelemetry-auto-instrumentation-go` in the containers.

1. Verify that traces appear in your trace backend.

1. If you configured `OTEL_METRICS_EXPORTER` to `prometheus`, verify that metrics are exposed. Port-forward the default metrics port `9464` and send a request to the endpoint:

   ```bash
   kubectl port-forward --namespace {NAMESPACE} pod/{POD_NAME} 9464:9464
   ```

   ```bash
   curl http://localhost:9464/metrics
   ```

## Clean Up

1. Remove the injection annotation from your workload's Pod template to stop agent injection.

1. Restart the Deployment to apply the change:

   ```bash
   kubectl rollout restart deployment/{DEPLOYMENT_NAME} --namespace {NAMESPACE}
   ```

1. Delete the Instrumentation CR:

   ```bash
   kubectl delete instrumentation my-instrumentation -n my-app
   ```

1. Uninstall the OTel Operator:

   ```bash
   helm uninstall my-opentelemetry-operator -n opentelemetry-operator-system
   ```

1. If you installed cert-manager only for this guide, remove it:

   ```bash
   helm uninstall cert-manager -n cert-manager
   ```
