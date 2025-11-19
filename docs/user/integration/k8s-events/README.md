# Integrate Kubernetes Events as OTLP Logs

## Overview

| Category| |
| - | - |
| Signal types | logs |
| Backend type | custom in-cluster, third-party remote |
| OTLP-native | yes |

Learn how to collect Kubernetes cluster events and forward them as OTLP  logs to your observability backend. You install and configure a custom [OTel Collector](https://github.com/open-telemetry/opentelemetry-collector) in your Kyma cluster using a provided [Helm chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector). The collector watches for Kubernetes events, converts them to OTLP logs, and forwards them to the Telemetry module's log gateway for further processing and enrichment.

> Note
> This guide uses the OpenTelemetry [`k8s-events`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8seventsreceiver) receiver. It is the recommended, modern receiver for this task because it provides a dedicated attribute scheme optimized for Kubernetes events.

![setup](./../assets/k8s-events.drawio.svg)

## Table of Content

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Clean Up](#clean-up)

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](../../README.md) is [added](https://kyma-project.io/#/02-get-started/01-quick-install)
- You have set up a `LogPipeline` to send logs to a backend, for example, by following the [SAP CLoud Logging guide](./../sap-cloud-logging/) or [Loki](./../loki/)
- [Kubectl version that is within one minor version (older or newer) of `kube-apiserver`](https://kubernetes.io/releases/version-skew-policy/#kubectl)
- Helm 3.x

## Installation

### Preparation

1. Export your namespace as a variable with the following command:

    ```bash
    export K8S_NAMESPACE="k8s-events"
    ```

1. If you haven't created a namespace yet, do it now:

    ```bash
    kubectl create namespace $K8S_NAMESPACE
    ```

1. Set the following label to enable Istio injection in your namespace:

    ```bash
    kubectl label namespace $K8S_NAMESPACE istio-injection=enabled
    ```

1. Export the Helm release name. The release name must be unique within the namespace. All resources in the cluster will be prefixed with this name.

    ```bash
    export HELM_RELEASE="k8s-events"
    ```

1. Update your Helm installation with the required Helm repository:

    ```bash
    helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
    helm repo update
    ```

## Install the Collector

Run the Helm upgrade command to deploy the collector. It installs the chart only if it is not present yet.

```bash
helm upgrade --install --create-namespace -n $K8S_NAMESPACE $HELM_RELEASE open-telemetry/opentelemetry-collector -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/k8s-events/values.yaml
```

<!-- markdown-link-check-disable -->
The command uses the [values.yaml](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/k8s-events/values.yaml) file that customizes the default chart settings:
<!-- markdown-link-check-enable -->

- Configures the deployment mode and the image to use
- Configure the log pipeline with the `k8s-events` receiver and an OTLP exporter shipping to the Kyma-specific endpoint
- Setup required RBAC and resource settings

Alternatively, you can create your own `values.yaml` file and adjust the command.

## Verify the Installation

To verify that the collector is running properly, set up port forwarding and call the respective local hosts.

1. Verify the collector starts up:

   ```bash
   kubectl -n $K8S_NAMESPACE get pods
   ```

1. Verify that logs arrive at your observability backend. The event logs contain the attribute `instrumentationScope.name: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver`.

## Integrate with SAP Cloud Logging (Optional)

If the LogPipeline of the Telemetry module is configured with a SAP Cloud Logging instance (see [Integrate with SAP Cloud Logging](./../sap-cloud-logging/), you can install a custom Search and Dashboard called `K8S Events` to explore the data.
For that, import the file [cloud-logging-dashboard.ndjson](https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sap-cloud-logging/dashboard-runtime.ndjson).

## Clean Up

To remove your custom OTel Collector and all its resources from the cluster, run the following Helm command:

```bash
helm delete -n $K8S_NAMESPACE $HELM_RELEASE
```
