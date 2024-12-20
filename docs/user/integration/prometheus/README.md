# Integrate with Prometheus

| Category     |                                         |
| ------------ | --------------------------------------- |
| Signal types | metrics                                 |
| Backend type | custom local                            |
| OTLP-native  | yes                                     |

Learn how to configure the Telemetry module to ingest metrics in a custom [Prometheus](https://prometheus.io/) instance deployed with the [`kube-prometheus-stack`](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack).

## Table of Content

- [Integrate with Prometheus](#integrate-with-prometheus)
  - [Table of Content](#table-of-content)
  - [Prerequisites](#prerequisites)
  - [Context](#context)

## Prerequisites

- Kyma as the target deployment environment.
- The [Telemetry module](../../README.md) is added. For details, see [Quick Install](https://kyma-project.io/#/02-get-started/01-quick-install). <!-- This link differs for OS and SKR -->
- If you want to use Istio access logs, make sure that the [Istio module](https://kyma-project.io/#/istio/user/README) is added.
<!-- markdown-link-check-disable -->
- Kubernetes CLI (kubectl) (see [Install the Kubernetes Command Line Tool](https://developers.sap.com/tutorials/cp-kyma-download-cli.html)).
<!-- markdown-link-check-enable -->
- UNIX shell or Windows Subsystem for Linux (WSL) to execute commands.

> [!WARNING]
- This guide describes a basic setup that you should not use in production. Typically, a production setup needs further configuration, like optimizing the amount of data to be collected and the required resource footprint of the installation. To achieve qualities like [high availability](https://prometheus.io/docs/introduction/faq/#can-prometheus-be-made-highly-available), [scalability](https://prometheus.io/docs/introduction/faq/#i-was-told-prometheus-doesnt-scale), or [durable long-term storage](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage), you need a more advanced setup.
- This example uses the latest Grafana version, which is under AGPL-3.0 and might not be free of charge for commercial usage.

## Context

The Telemetry module supports shipping metrics from applications and the Istio service mesh to Prometheus using the OpenTelemetry protocol (OTLP). Prometheus is a widely used backend for collection and storage of metrics. The `kube-prometheus-stack` is a Helm chart which bundles Prometheus together with Grafana and the Alertmanager, to have an instant and comprehensive monitoring experience. Furthermore, it brings community-driven best practices on kubernetes monitoring including the node-exporter and kube-state-metrics component

As the OpenTelemetry community is not that advanced yet in providing a full-blown Kubernetes monitoring, this guide shows how to combine the two worlds by integrating application and Istio metrics via the telemetry module, and the kubernetes monitoring via the features of the bundle.

In this guide you will first deploy the `kube-prometheus-stack`, then you will configure the telemetry module to start metric ingestion, and then you deploy the sample application to illustrate custom metric consumption.

![setup](./../assets/prometheus.drawio.svg)

## Guide

## Install the kube-prometheus-stack

1. Export your Namespace as a variable. Replace the `{namespace}` placeholder in the following command and run it:

    ```bash
    export K8S_PROM_NAMESPACE="{namespace}"
    ```
1. If you haven't created the Namespace yet, now is the time to do so:
    ```bash
    kubectl create namespace $K8S_PROM_NAMESPACE
    ```
   >**Note**: This Namespace must have **no** Istio sidecar injection enabled; that is, there must be no `istio-injection` label present on the Namespace. The Helm chart deploys jobs that will not succeed when Isto sidecar injection is enabled.

1. Export the Helm release name that you want to use. It can be any name, but be aware that all resources in the cluster will be prefixed with that name. Run the following command:
    ```bash
    export HELM_PROM_RELEASE="prometheus"
    ```

1. Update your Helm installation with the required Helm repository:

    ```bash
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    ```

1. Run the Helm upgrade command, which installs the chart if it's not present yet. At the end of the command, change the Grafana admin password to some value of your choice.
    ```bash
    helm upgrade --install -n ${K8S_PROM_NAMESPACE} ${HELM_PROM_RELEASE} prometheus-community/kube-prometheus-stack -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/prometheus/values.yaml --set grafana.adminPassword=myPwd
    ```

1. You can use the [values.yaml](./values.yaml) provided with this tutorial, which contains customized settings deviating from the default settings, or create your own one.
The provided `values.yaml` covers the following adjustments:
- Basic Istio setup to secure communication between Prometheus, Grafana and Alertmanager
- Native OTLP receiver enabled for prometheus
- Basic configuration of data persistence with retention
- Basic resource limits for involved components

## Verify the kube-prometheus-stack

1. You should see several Pods coming up in the Namespace, especially Prometheus, Grafana and Alertmanager. Assure that all Pods have the "Running" state.
2. Browse the Prometheus dashboard and verify that all "Status->Targets" are healthy. The following command exposes the dashboard on `http://localhost:9090`:
   ```bash
   kubectl -n ${K8S_PROM_NAMESPACE} port-forward $(kubectl -n ${K8S_PROM_NAMESPACE} get service -l app=kube-prometheus-stack-prometheus -oname) 9090
   ```
3. Browse the Grafana dashboard and verify that the dashboards are showing data. The user `admin` is preconfigured in the Helm chart; the password was provided in your `helm install` command. The following command exposes the dashboard on `http://localhost:3000`:
   ```bash
   kubectl -n ${K8S_PROM_NAMESPACE} port-forward svc/${HELM_PROM_RELEASE}-grafana 3000:80
   ```

## Activate a MetricPipeline

1. Apply a MetricPipeline resource having the output configured with the local prometheus URL and having the inputs enabled for collecting Istio metrics and collecting application metrics whose workloads are annotated with prometheus annotations.
```yaml
SERVICE=$(kubectl -n ${K8S_PROM_NAMESPACE} get service -l app=kube-prometheus-stack-prometheus -ojsonpath='{.items[*].metadata.name}')
kubectl apply -n sap-cloud-logging-integration -f - <<EOF
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
    name: prometheus
spec:
    input:
        prometheus:
            enabled: true
            namespaces:
                exclude:
                - kyma-system
        istio:
            enabled: true
    output:
        otlp:
            protocol: http
            endpoint:
                value: "http://${SERVICE}.${K8S_PROM_NAMESPACE}:9090/api/v1/otlp"
EOF
```

1. Verify the MetricPipeline health by verifying that all attributes are "true":
    ```sh
    kubectl get metricpipeline prometheus
    ```

1. Port forward to Grafana once more and verify in the "Explore" view if metrics with prefix "istio_" are available.

1. Optionally import the Istio Grafana dashboards as described [here](https://istio.io/latest/docs/ops/integrations/grafana/#option-2-import-from-grafanacom-into-an-existing-deployment) and verify that the dashboards are showing data

## Deploy the sample application

1. Deploy the [sample app](./../sample-app/):

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/sample-app/deployment/deployment.yaml -n $K8S_PROM_NAMESPACE
    ```

1. Go to your Grafana Dashboard as described above and check in the explore view for the metric `cpu_temperature_celsius` which is emmited by the sample app

### Cleanup

1. To remove the installation from the cluster, call Helm:
    ```bash
    helm delete -n ${K8S_PROM_NAMESPACE} ${HELM_PROM_RELEASE}
    ```

1. To remove the MetricPipeline, call kubectl:
    ```bash
    helm delete MetricPipeline prometheus
    ```

1. Run the following command to completely remove the example app and all its resources from the cluster:
    ```bash
    kubectl delete all -l kubernetes.io/name=sample-app -n $K8S_PROM_NAMESPACE
    ```
