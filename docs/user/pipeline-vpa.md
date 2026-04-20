# Vertical Pod Autoscaler (VPA) for Telemetry Pipelines

## Overview

The Telemetry module automatically adjusts memory resources for the OTLP Gateway and telemetry agents based on actual usage patterns. This dynamic resource management ensures efficient resource utilization while preventing out-of-memory issues.

## How VPA Works

Vertical Pod Autoscaler (VPA) monitors the resource consumption of your telemetry components and automatically adjusts their memory requests and limits. When you create telemetry pipelines (LogPipeline, TracePipeline, or MetricPipeline), Telemetry Manager deploys the necessary components with VPA enabled by default.

VPA provides the following benefits:

- **Automatic Resource Optimization**: Adjusts memory allocation based on actual workload, preventing both resource waste and out-of-memory conditions.
- **Dynamic Scaling**: Responds to changes in telemetry volume as your workloads and pipelines evolve.
- **Reduced Manual Tuning**: Eliminates the need to manually configure resource requests and limits for telemetry components.

## VPA Requirements

To use VPA with telemetry pipelines, ensure that:

- The Vertical Pod Autoscaler CRD is installed in your cluster. Most managed Kubernetes distributions (for example, GKE, EKS, AKS) provide VPA as an optional feature.
- Cluster nodes have sufficient allocatable memory to accommodate resource adjustments.

> [!NOTE]
> If the VPA CRD is not available in your cluster, Telemetry Manager automatically falls back to static resource configuration. Your pipelines continue to work without VPA.

## Default Behavior

VPA is **enabled by default** for all telemetry components when you create a pipeline. No additional configuration is required.

When VPA is active:

- Memory requests start at a baseline value and adjust based on actual usage.
- Memory limits are set to 2x the memory request, allowing VPA to scale within a safe range.
- The VPA maxAllowed memory is automatically calculated as 30% of the smallest node's allocatable memory, rounded down to the nearest KiB.

## Disable VPA

To disable VPA for all telemetry pipelines, add an annotation to the Telemetry custom resource:

1. Edit the Telemetry resource:

   ```bash
   kubectl edit telemetry default -n kyma-system
   ```

2. Add the following annotation under `metadata.annotations`:

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

3. Save and close the editor.

After you disable VPA, telemetry components use static resource configurations. The OTLP Gateway and agents scale their memory requests based on the number of active pipelines using a fixed multiplier.

> [!WARNING]
> Disabling VPA removes automatic memory adjustment. Monitor your telemetry components to ensure they have sufficient resources for your workload.

## Re-enable VPA

To re-enable VPA after disabling it:

1. Edit the Telemetry resource:

   ```bash
   kubectl edit telemetry default -n kyma-system
   ```

2. Remove the `telemetry.kyma-project.io/enable-vpa: "false"` annotation, or change its value to `"true"`:

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

3. Save and close the editor.

Telemetry Manager recreates the VPA resources and resumes automatic memory management.

## Verify VPA Status

To check if VPA is managing your telemetry components:

1. List VPA resources in the `kyma-system` namespace:

   ```bash
   kubectl get vpa -n kyma-system
   ```

   You should see VPA resources for active telemetry components:

   ```
   NAME                          MODE              CPU    MEM       PROVIDED   AGE
   telemetry-otlp-gateway        InPlaceOrRecreate        128Mi     True       10m
   telemetry-log-agent           InPlaceOrRecreate        64Mi      True       10m
   telemetry-metric-agent        InPlaceOrRecreate        64Mi      True       10m
   ```

2. Check VPA recommendations for a specific component:

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
