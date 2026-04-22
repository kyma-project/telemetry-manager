# Manage Automatic Resource Scaling

By default, the Telemetry module adjusts memory resources for the OTLP Gateway and agents using Vertical Pod Autoscaler (VPA). You can disable this automatic scaling if you need predictable resource allocation or want to manage resources differently.

## Prerequisites

- Cluster Nodes have sufficient allocatable memory to accommodate resource adjustments.
- The VPA Custom Resource Definition (CRD) is installed in your cluster. Most managed Kubernetes distributions (for example, GKE, EKS, AKS) provide VPA as an optional feature. For installation instructions, see [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler). If the VPA CRD is not available, Telemetry Manager automatically falls back to static resource configuration. Your pipelines continue to work.

## Context

The Telemetry module configures VPA with the following settings:

- Memory limits are set to 2x the memory request.
- Maximum allowed memory is calculated as 30% of the smallest Node's allocatable memory, rounded down to the nearest KiB.

When you upgrade the Telemetry module to a version with VPA support, VPA activates for existing Telemetry components. This triggers a one-time restart of Telemetry components as VPA applies its initial resource recommendations. During the initial adjustment period, you might observe Pod restarts as VPA learns your workload patterns. This is normal behavior.

## Procedure

> [!NOTE]
> Disabling automatic scaling removes dynamic memory adjustment. Monitor your telemetry components to ensure they have sufficient resources for your workload. For monitoring instructions, see [Monitor Pipeline Health](./monitor-pipeline-health.md).

1. To disable automatic scaling, edit the Telemetry resource:

   ```bash
   kubectl edit telemetry default -n kyma-system
   ```

2. Add or change the `telemetry.kyma-project.io/enable-vpa` annotation under `metadata.annotations` to `"false"`:

   ```yaml
   apiVersion: operator.kyma-project.io/v1beta1
   kind: Telemetry
   metadata:
     name: default
     namespace: kyma-system
     annotations:
       telemetry.kyma-project.io/enable-vpa: "false"
   spec:
     # your telemetry configuration
   ```

3. Save your changes.

4. Verify the configuration by listing VPA resources in the `kyma-system` namespace:

   ```bash
   kubectl get vpa -n kyma-system
   ```

   If automatic scaling is enabled, VPA resources for active telemetry components appear in the output:

   ```text
   NAME                          MODE              CPU    MEM       PROVIDED   AGE
   telemetry-otlp-gateway        InPlaceOrRecreate        128Mi     True       10m
   telemetry-log-agent           InPlaceOrRecreate        64Mi      True       10m
   telemetry-metric-agent        InPlaceOrRecreate        64Mi      True       10m
   ```

   If automatic scaling is disabled, no VPA resources appear in the namespace.

5. To check VPA recommendations for a specific component:

   ```bash
   kubectl describe vpa telemetry-otlp-gateway -n kyma-system
   ```

   The output shows current resource usage and VPA recommendations:

   ```yaml
   Status:
     Recommendation:
       Container Recommendations:
         Container Name:  collector
         Lower Bound:
           Memory:  64Mi
         Target:
           Memory:  128Mi
         Upper Bound:
           Memory:  256Mi
   ```

## Result

When VPA is disabled, Telemetry components use static resource configurations.

## Next Steps

To re-enable automatic scaling, remove the annotation or change its value to `"true"`:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
  annotations:
    telemetry.kyma-project.io/enable-vpa: "true"
spec:
  # your telemetry configuration
```

Telemetry Manager recreates the VPA resources and resumes automatic memory management.
