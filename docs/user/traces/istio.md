# Configure Istio Tracing

Use the Istio Telemetry API to selectively enable integration of Istio traces with the Telemetry module.

## Prerequisites

* You have the Istio module added.
* To use CLI instruction, you must install [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) and [curl](https://curl.se/). Alternatively, you can use Kyma dashboard.

## Context

The Istio module is crucial in distributed tracing because it provides the [Ingress Gateway](https://istio.io/latest/docs/tasks/traffic-management/ingress/ingress-control/). Typically, this is where external requests enter the cluster scope and are enriched with trace context if it hasn’t happened earlier. Furthermore, every component that’s part of the Istio Service Mesh runs an Istio proxy, which propagates the context properly but also creates span data. If Istio tracing is activated and taking care of trace propagation in your application, you get a complete picture of a trace, because every component automatically contributes span data. Also, Istio tracing is pre-configured to be based on the vendor-neutral [W3C Trace Context](https://www.w3.org/TR/trace-context/) protocol.

The Istio module is configured with an [extension provider](https://istio.io/latest/docs/tasks/observability/telemetry/) called `kyma-traces`. To activate the provider on the global mesh level using the Istio [Telemetry API](https://istio.io/latest/docs/reference/config/telemetry/#Tracing), place a resource to the `istio-system` namespace. The following code samples help setting up the Istio tracing feature:

> [!WARNING]
> Enabling Istio traces may drastically increase data volume and might quickly fill up your trace storage.

See also [Kyma Access Logs with Istio](./../logs/README.md#istio) for more details on how to enable Istio access logs using the same API.

## Configuration

Use the Telemetry API to selectively enable Istio tracing. See:

<!-- no toc -->
* [Configure Istio Tracing for the Entire Mesh](#configure-istio-tracing-for-the-entire-mesh)
* [Configure a Sampling Rate](#configure-a-sampling-rate)
* [Configure Istio Tracing for Namespaces or Workloads](#configure-istio-tracing-for-namespaces-or-workloads)
* [Propagate Trace Context Without reporting Spans](#propagate-trace-context-without-reporting-spans)

### Configure Istio Tracing for the Entire Mesh

The following example configures all Istio proxies with the `kyma-traces` extension provider, which, by default, reports span data to the trace gateway of the Telemetry module.

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
    randomSamplingPercentage: 5.00
```

> [!NOTE]
> There can be only one Istio Telemetry resource on global mesh level. If you also enable [Kyma access logs with Istio](./../logs/README.md#istio), assure that the configuration happens in the same resource.

### Configure a Sampling Rate

By default, the sampling rate is configured to 1%. That means that only 1 trace out of 100 traces is reported to the trace gateway, and all others are dropped. The sampling decision itself is propagated as part of the [trace context](https://www.w3.org/TR/trace-context/#sampled-flag) so that either all involved components are reporting the span data of a trace, or none.

> [!TIP]
> If you increase the sampling rate, you send more data your tracing backend and cause much higher network utilization in the cluster.
> To reduce costs and performance impacts in a production setup, a very low percentage of around 5% is recommended.

To configure an "always-on" sampling, set the sampling rate to 100%:

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
    randomSamplingPercentage: 100.00
```

### Configure Istio Tracing for Namespaces or Workloads

If you need specific settings for individual namespaces or workloads, place additional Telemetry resources. If you don't want to report spans at all for a specific workload, activate the `disableSpanReporting` flag with the selector expression.

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: tracing
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: "my-app"
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 100.00
```

### Propagate Trace Context Without reporting Spans

To enable the propagation of the [W3C Trace Context](https://www.w3.org/TR/trace-context/) only, without reporting any spans (so the actual tracing feature is disabled), you must enable the `kyma-traces` provider with a sampling rate of 0. With this configuration, you get the relevant trace context into the [access logs](https://kyma-project.io/#/istio/user/tutorials/01-45-enable-istio-access-logs) without any active trace reporting.

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
      randomSamplingPercentage: 0
  ```
