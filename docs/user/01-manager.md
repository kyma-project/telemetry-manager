# Telemetry Manager

As the core element of the Telemetry module, Telemetry Manager manages the lifecycle of other Telemetry module components by watching user-created resources.

## Module Lifecycle

The Telemetry module includes Telemetry Manager, a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that's described by a custom resource of type Telemetry. Telemetry Manager has the following tasks:

- Watch for the user-created Kubernetes resources LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
- If it finds such a custom resource: Roll out the relevant components on demand and keep it in sync with the pipeline.

![Manager](assets/manager-lifecycle.drawio.svg)

## Module Configuration
<!--- This content differs from DITA, in clarification --->
In the [Telemetry resource](resources/01-telemetry.md), you can configure the number of replicas for the `telemetry-trace-gateway` and `telemetry-metric-gateway` deployments. The default value is 2.

```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  trace:
    gateway:
      scaling:
        type: Static
        static:
          replicas: 3
  metric:
    gateway:
      scaling:
        type: Static
        static:
          replicas: 4
```

## Module Status

Telemetry Manager syncs the overall status of the module into the [Telemetry resource](resources/01-telemetry.md); it can be found in the `status` section.
