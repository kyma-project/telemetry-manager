# 13. Leader Receiver Creator as Extension

Date: 2024-11-08

## Status
Proposed

## Context

The Telemetry OTel Collector collects metrics exposed by the Kubernetes API server using [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver), 
and custom resource metrics are collected using [kymastatsreceiver](https://github.com/kyma-project/opentelemetry-collector-components/tree/main/receiver/kymastatsreceiver). 
To run these receivers in high availability mode and prevent sending duplicate metrics, we've implemented [Singleton Receiver Creator](https://github.com/kyma-project/opentelemetry-collector-components/tree/main/receiver/singletonreceivercreator) based on [Leader Receiver Creator](./012-leader-receiver-creator.md).

## Decision
However, after feedback from community and careful deliberation, we realize the `singleonreceivercreator` would be better suited to be used as an extension.
[Extensions](https://github.com/open-telemetry/opentelemetry-collector/blob/main/extension/README.md?plain=1) provide additional functionality to the collector, but do not need direct access to telemetry data and are not part of the pipelines.

Implementing `singleonreceivercreator` as an extension brings the following advantages:
- It is signal-agnostic.
- It can be used with any receiver.
- It brings simpler and clear configuration.
- It's future-proof.

It has following disadvantages:
- The receiver must be modified to support the leader election extension.


### Leader Election API

```yaml
receivers:
  dummy/foo:
    interval: 1m
    leaderelector: leaderelector/foo
  dummy/bar:
    interval: 1m
    leaderelector: leaderelector/bar
extensions:
  leaderelector/foo:
    auth_type: kubeConfig
    lease_name: foo
    lease_namespace: default
  leaderelector/bar:
    auth_type: kubeConfig
    lease_name: bar
    lease_namespace: default
```

The leader elector extension would contain the configuration providing lease name and namespace. This extension would be then referenced in the receivers.

### Consequences
The Singleton Receiver Creator (https://github.com/kyma-project/opentelemetry-collector-components/tree/main/receiver/singletonreceivercreator) would be deprecated and removed. The `kymastatsreceiver` and `k8sclusterreceiver` would use the leader elector extension for leader election.

The change would also require changes in the `k8sclusterreceiver` to support the leader election extension.


