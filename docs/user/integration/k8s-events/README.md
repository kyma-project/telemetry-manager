# Integrate Kubernetes Events using an OTel Collector

## Overview

| Category| |
| - | - |
| Signal types | logs |
| Backend type | custom in-cluster, third-party remote |
| OTLP-native | yes |

Learn how to integrate a custom OTel Collector to add Kubernetes events watched and expoted as OTLP Logs using the telemetry module. For that you will install and configure a custom OpenTelemetry [OTel Collector](https://github.com/open-telemetry/opentelemetry-collector) in a Kyma cluster using a provided [Helm chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector). The collector will be configured to push log data using OTLP to the collector that's provided by Kyma, so that they are collected and enriched together with any other log.

> Note
> The OTel Collector community has two approaches for watching events, the [`k8s-events`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8seventsreceiver) receiver and the [`k8s-objects`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver) receiver. For a long time, the `k8s-events` receiver was deprecated as there was the hope to have it fully covered by the more generic `k8s-objects` receiver, which even is used by the presets available for event collection in the used Helm chart. However, it recently crystalized, that this topic requires a dedicated receiver with a dedicated attribute scheme which makes it way more simple to make use of the produced data, and this guide will be based on the `k8s-event` receiver.

![setup](./../assets/k8s-events.drawio.svg)

## Table of Content

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Clean Up](#clean-up)

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](../../README.md) is [added](https://kyma-project.io/#/02-get-started/01-quick-install)
- The [Telemetry module](../../README.md) is configured with pipelines logs, for example, by following the [SAP CLoud Logging guide](./../sap-cloud-logging/) or [Loki](./../loki/)
- [Kubectl version that is within one minor version (older or newer) of `kube-apiserver`](https://kubernetes.io/releases/version-skew-policy/#kubectl)
- Helm 3.x

## Installation

### Preparation

1. Export your namespace as a variable with the following command:

    ```bash
    export K8S_NAMESPACE="k8s-events"
    ```

1. If you haven't created a Namespace yet, do it now:

    ```bash
    kubectl create namespace $K8S_NAMESPACE
    ```

1. Set the following label to enable Istio injection in your Namespace:

    ```bash
    kubectl label namespace $K8S_NAMESPACE istio-injection=enabled
    ```

1. Export the Helm release name that you want to use. The release name must be unique for the chosen Namespace. Be aware that all resources in the cluster will be prefixed with that name. Run the following command:

    ```bash
    export HELM_RELEASE="k8s-events"
    ```

1. Update your Helm installation with the required Helm repository:

    ```bash
    helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
    helm repo update
    ```

### Install the Collector

Run the Helm upgrade command. It installs the chart only if it is not present yet.

```bash
helm upgrade --install --create-namespace -n $K8S_NAMESPACE $HELM_RELEASE open-telemetry/opentelemetry-collector -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/k8s-events/values.yaml
```

<!-- markdown-link-check-disable -->
The previous command uses the [values.yaml](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/k8s-events/values.yaml), which contains customized settings deviating from the default settings. The customizations in the provided `values.yaml` cover the following areas:
<!-- markdown-link-check-enable -->

- Configures the deployment mode and the image to use
- Configure the log pipeline with the k8s-events receiver and an OTLP exporter shipping to the Kyma specific endpoint
- Setup proper RBAC and resource settings

Alternatively, you can create your own `values.yaml` file and adjust the command.

### Verify the Setup

To verify that the collector is running properly, set up port forwarding and call the respective local hosts.

1. Verify the collector starts up:

   ```bash
   kubectl -n $K8S_NAMESPACE get pods
   ```

1. Verify that logs arrive at the backend. The logs will have the `instrumentationScope.name: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver`.

### SAP Cloud Logging Integration

If the LogPipeline of the telemetry module is configured with a SAP Cloud Logging instance as described in the [SAP CLoud Logging guide](./../sap-cloud-logging/), you can install a custom Search and Dashboard called `K8S Events` to explore the data.
For that, import the file [cloud-logging-dashboard.ndjson](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-runtime.ndjson).

## Clean Up

When you're done, you can remove the collector and all its resources from the cluster by calling Helm:

```bash
helm delete -n $K8S_NAMESPACE $HELM_RELEASE
```
