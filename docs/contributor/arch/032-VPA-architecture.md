# 32. Vertical Pod Autoscaler (VPA) Architecture

## Context
Central OTLP Gateway is DaemonSet based deployment. When the load increases the pod should be scaled up to handle the load.
Vertical Pod Autoscaler (VPA) is a Kubernetes component that automatically adjusts the resource requests and limits of pods based on their actual usage.

VerticalPodAutoscaler CRD stores the recommendations based on metrics from metric server. This recommendation is used by VPA Updater to update
or recreate Deployment or DaemonSet. Additionally There is VPA Admission Controller, a mutating webhook which injects the new resource values. Refer [1] for
detailed architecture of VPA.

## Proposal
### Option 1: Allow VPA Updater to update the Central OTLP Gateway DaemonSet directly
The option allows VPA updater to update the resources in the pods. We have to use VPA with `updateMode: "InPlaceOrRecreate"` to allow VPA to update the resources in the pods.

![Option 1 VPA Updater updates DaemonSet directly](../assets/032-vpa-updater.svg)
Pros:
 - Stability as VPA updates the pods taking into account various factors such as: Priority Class, Pod Disruption Budget, etc.
 - The complex logic of making a decision to update the pod resources is handled by VPA.
 - As the pods resources are updated via mutating webhook, the DaemonSet definition is not changed. Hence, there is no new reconciliation triggered.
Cons:
 - The DaemonSet definition would not show the current status of resource usage as the Pods resources are update by mutation webhook (VPA Admission Controller).


### Option 2: Our reconciler updates the Central OTLP Gateway DaemonSet based on VPA recommendations
In this option, our reconciler will watch the VerticalPodAutoscaler CR and update the Central OTLP Gateway DaemonSet based on the recommendations provided by VPA.

![Option 2 Reconciler updates DaemonSet based on VPA recommendations](../assets/032-vpa-reconciler.svg)
Pros:
 - The DaemonSet definition would show the current status of resource usage as the reconciler updates the DaemonSet directly.
Cons:
 - The logic to decide when to update the DaemonSet based on VPA recommendations would be complex and needs to implemented in reconciler. Although reconciler might not need to take into account Pod Disruption Budget and Priority Class.



## Caveats
- VPA Crd should be used to update requests only. The reason being the limits are calculated based on ratio of limits/requests in base DaemonSet Spec. In our case its (2000Mi/32Mi=62.5). So the VPA updater will update limits as 62.5 x current request recommendation which would be larger memory than node has to offer.
- The scale down decision is made based on long time historical data. So the scaled down would take some time.
- GOMEMLIMIT can be based on max memory limit (2Gi).

## Decision
- We will go with.......
- We will use VPA with `controlledValues: Requests`.