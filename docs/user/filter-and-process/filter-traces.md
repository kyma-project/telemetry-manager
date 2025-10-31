# Filter Traces

TracePipeline resources have no `input` specification. You can configure Istio trace collection by applying the Istio `Telemetry` resource to specific namespaces.

## Override Tracing for a Namespace or Workload

After setting a mesh-wide default, apply more specific tracing configurations for an entire namespace or for individual workloads within a namespace. 

To do this, create an Istio `Telemetry` resource in the workload's namespace. To apply a tracing configuration to a specific workload within the namespace, add a `selector` that matches the workload's labels.

For example, increase the sampling rate (in this example, to `100.00`) to debug a specific application without affecting the entire mesh:

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: tracing
  namespace: $YOUR_NAMESPACE
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: "my-app"
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 100.00
```

## Disable Tracing for a Specific Workload

To completely disable Istio span reporting for a specific workload while keeping it enabled for the rest of the mesh, create a `Telemetry` resource that targets the workload and set `disableSpanReporting` to `true`.

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: $YOUR_APP_NAME-tracing-disable
  namespace: $YOUR_NAMESPACE
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: "$YOUR_APP_NAME"
  tracing:
  - providers:
    - name: "kyma-traces"
    disableSpanReporting: true
```
