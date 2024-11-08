# 13. Leader Receiver Creator as Extension

Date: 2024-11-08

## Status
Proposed

## Context

Telemetry OTeL collector collects metrics exposed via Kubernetes API server via [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver) 
and custom resource metrics are collected via [kymastatsreceiver](https://github.com/kyma-project/opentelemetry-collector-components/tree/main/receiver/kymastatsreceiver). In order to run these receiver 
in high availability mode and prevent sending duplicate metrics, [singletonreceivercreator](https://github.com/kyma-project/opentelemetry-collector-components/tree/main/receiver/singletonreceivercreator)
was implemented based on [leader receiver creator](./012-leader-receiver-creator.md).

## Decision
However, after feedback from community and careful deliberation the `singleonreceivercreator` would be better suited to be
used as an extension. [Extensions](https://github.com/open-telemetry/opentelemetry-collector/blob/main/extension/README.md?plain=1) are used to provide additional functionality to the collector. These components do not need direct access to
telemetry data and are not part of the pipelines.

Implementing it as an extension brings following advantages:
- It is signal agnostic
- It can be used with any receiver
- Simpler and clear configuration
- Future-proof

It has following disadvantages:
- The receiver needs to be modified to support leader election extension


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

The leader elector extension would contain the configuration providing lease name and namespace. This extension would be
then referenced in the receiver

### Consequences
The `singletonreceivercreator` (https://github.com/kyma-project/opentelemetry-collector-components/tree/main/receiver/singletonreceivercreator) would be deprecated and removed. The kymastatsreceover and 
`k8sclusterreceiver` would use the leader elector extension for leader election.

It would also require changes in the `k8sclusterreceiver` to support leader election extension.


