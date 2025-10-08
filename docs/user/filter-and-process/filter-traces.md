# Filter Traces

`TracePipeline` resources have no `input` specification. You can configure Istio trace collection by applying the Istio `Telemetry` resource to specific namespaces.

## Override Tracing for a Namespace or Workload

After setting a mesh-wide default, apply more specific tracing configurations for an entire namespace or for individual workloads within a namespace. Use this to debug a particular service by increasing its sampling rate without affecting the entire mesh.

To do this, create a `Telemetry` resource in the workload's namespace. To apply a tracing configuration to a specific workload within the namespace, add a `selector` that matches the workload's labels.

1. Export the name of the workload's namespace and application name as environment variables:

   ```bash
   export YOUR_NAMESPACE=<NAMESPACE_NAME>
   export YOUR_APP_NAME=<APP_NAME>
   ```

2. Apply the `Telemetry` resource with the `selector`. The following example increases the sampling rate to `100.00` only for the target app.

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

3. Verify that the resource is applied to the target namespace:

   ```bash
   kubectl -n $YOUR_NAMESPACE get telemetries.telemetry.istio.io
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