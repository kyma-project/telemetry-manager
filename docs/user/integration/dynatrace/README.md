# Integrate With Dynatrace

## Overview

| Category| |
| - | - |
| Signal types | traces, metrics |
| Backend type | third-party remote |
| OTLP-native | yes |

[Dynatrace](https://www.dynatrace.com) is an advanced Application Performance Management solution available as SaaS offering. It supports monitoring both the Kubernetes cluster itself and the workloads running in the cluster. To use all of its features, the proprietary agent technology of Dynatrace must be installed.

Combined with the Kyma Telemetry module, you can collect custom spans and metrics, as well as Istio spans and metrics and ship them to Dynatrace. Get an introduction on how to set up Dynatrace and learn how to integrate the Kyma Telemetry module.

![setup](./../assets/dynatrace.drawio.svg)

## Table of Content

- [Prerequisites](#prerequisites)
- [Prepare the Namespace](#prepare-the-namespace)
- [Dynatrace Setup](#dynatrace-setup)
- [Telemetry Module Setup](#telemetry-module-setup)
  - [Create Secret](#create-secret)
  - [Ingest Traces](#ingest-traces)
  - [Ingest Metrics](#ingest-metrics)
- [Set Up Kyma Dashboard Integration](#set-up-kyma-dashboard-integration)
- [Use Dynatrace Dashboards](#use-dynatrace-dashboards)
- [Use Dynatrace Alerts](#use-dynatrace-alerts)

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](https://kyma-project.io/#/telemetry-manager/user/README) is [added](https://kyma-project.io/#/02-get-started/01-quick-install)
- Active Dynatrace environment with permissions to create new access tokens
- Helm 3.x if you want to deploy the [OpenTelemetry sample application](../opentelemetry-demo/README.md)

## Prepare the Namespace

1. Export the namespace you want to use for Dynatrace as a variable with the following command:

    ```bash
    export DYNATRACE_NS="dynatrace"
    ```

1. If you haven't created a namespace yet, do it now:

    ```bash
    kubectl create namespace $DYNATRACE_NS
    ```

## Dynatrace Setup

To integrate Dynatrace, you first install the [Dynatrace Operator](https://github.com/Dynatrace/dynatrace-operator) and then create a `DynaKube` custom resource (CR). This CR configures the operator to roll out the OneAgent, which handles data collection. The Dynatrace OneAgent offers several [observability modes](https://docs.dynatrace.com/docs/ingest-from/setup-on-k8s/deployment#observability-options), two of which are relevant for application telemetry: `cloudNativeFullStack` and `applicationMonitoring`. Choose the deployment mode that fits your needs and apply the Kyma-specific configurations.

> [!NOTE]
> The following examples use API version `dynatrace.com/v1beta5`, which is also compatible with `v1beta4` and `v1beta3`. If you use an older version of the Dynatrace Operator, follow the [Migration guides for DynaKube apiVersions](https://docs.dynatrace.com/docs/ingest-from/setup-on-k8s/guides/migration/dynakube).

1. Install the Dynatrace operator with the namespace you prepared.

1. Create a [DynaKube](https://docs.dynatrace.com/docs/ingest-from/setup-on-k8s/reference/dynakube-parameters) CR with the `apiUrl` of your Dynatrace environment, and configure the `namespaceSelector` for both `metadataEnrichment` and for your desired observability mode:

   - Example for full-stack visibility:

     ```yaml
     apiVersion: dynatrace.com/v1beta5
     kind: DynaKube
     metadata:
       name: e2e-cluster
     spec:
       apiUrl: https://{YOUR_ENVIRONMENT_ID}.live.dynatrace.com/api
       metadataEnrichment:
         enabled: true
         namespaceSelector:
           matchExpressions:
           - key: operator.kyma-project.io/managed-by
             operator: NotIn
             values:
               - kyma
       oneAgent:
         cloudNativeFullStack:
           namespaceSelector:
             matchExpressions:
             - key: operator.kyma-project.io/managed-by
               operator: NotIn
               values:
                 - kyma
     ```

   - Example for application-level observability:

     ```yaml
     apiVersion: dynatrace.com/v1beta5
     kind: DynaKube
     metadata:
       name: e2e-cluster
     spec:
       apiUrl: https://{YOUR_ENVIRONMENT_ID}.live.dynatrace.com/api
       metadataEnrichment:
         enabled: true
         namespaceSelector:
           matchExpressions:
           - key: operator.kyma-project.io/managed-by
             operator: NotIn
             values:
               - kyma
       oneAgent:
         applicationMonitoring:
           namespaceSelector:
             matchExpressions:
             - key: operator.kyma-project.io/managed-by
               operator: NotIn
               values:
                 - kyma
     ```

1. Optionally, modify your DynaKube CR to enable OTLP ingestion using the OTel Collector (see [Enable Dynatrace telemetry ingest endpoints](https://docs.dynatrace.com/managed/ingest-from/setup-on-k8s/extend-observability-k8s/telemetry-ingest)):

    ```yaml
    spec:
      telemetryIngest:
        protocols:
        - otlp
      templates:
        otelCollector:
          imageRef:
            repository: public.ecr.aws/dynatrace/dynatrace-otel-collector
            tag: latest
    ```

1. In your Dynatrace UI, configure the Kubernetes integration settings (see [Dynatrace: Global default monitoring settings for Kubernetes/OpenShift](https://docs.dynatrace.com/docs/observe/infrastructure-monitoring/container-platform-monitoring/kubernetes-monitoring/default-monitoring-settings#configure-environment-level-settings)).

1. In the Dynatrace Hub, enable the **Istio Service Mesh** extension and follow the instructions to annotate your services.

After applying the `DynaKube` CR, the Dynatrace Operator deploys the necessary components, and you see data arriving in your Dynatrace environment.

## Telemetry Module Setup

Next, you set up the ingestion of custom spans and metrics, as well as Istio span and metric data.

### Create Secret

1. To push custom metrics and spans to Dynatrace, set up a [dataIngestToken](https://docs.dynatrace.com/docs/manage/identity-access-management/access-tokens-and-oauth-clients/access-tokens/personal-access-token).

   Follow the instructions in [Dynatrace: Generate an access token](https://docs.dynatrace.com/docs/manage/identity-access-management/access-tokens-and-oauth-clients/access-tokens/personal-access-token#generate-personal-access-tokens) and select the following scopes:

   - **Ingest metrics**
   - **Ingest OpenTelemetry traces**

2. Create an [apiToken](https://docs.dynatrace.com/docs/manage/identity-access-management/access-tokens-and-oauth-clients/access-tokens/personal-access-token) by selecting the template `Kubernetes: Dynatrace Operator`.

3. To create a new Secret containing your access tokens, replace the `<API_TOKEN>` and `<DATA_INGEST_TOKEN>` placeholder with the `apiToken` and `dataIngestToken` you created, replace the `<API_URL>` placeholder with the Dynatrace endpoint, and run the following command:

   ```bash
   kubectl -n $DYNATRACE_NS create secret generic dynakube --from-literal="apiToken=<API_TOKEN>" --from-literal="dataIngestToken=<DATA_INGEST_TOKEN>" --from-literal="apiurl=<API_URL>"
   ```

4. Verify the Secret you created looks similar to the [example Secret](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/integration/dynatrace/secret-example.yaml).

### Ingest Traces

To ingest custom spans, first deploy a TracePipeline. You can then optionally enable the Istio tracing feature to ingest Istio spans.
We recommend direct integration with the Dynatrace server. This approach reduces the number of components processing your trace data, improving resource efficiency and data shipment resiliency. Alternatively, you can integrate using the Dynatrace OpenTelemetry (OTel) Collector. Apply the same output configuration as described in [Ingest Metrics](#ingest-metrics).

1. Deploy the [TracePipeline](./../../collecting-traces/README.md):

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: TracePipeline
    metadata:
        name: dynatrace
    spec:
      transform:
        - statements:
          - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.statefulset.name"]) where IsString(resource.attributes["k8s.statefulset.name"])
          - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.replicaset.name"]) where IsString(resource.attributes["k8s.replicaset.name"])
          - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.job.name"]) where IsString(resource.attributes["k8s.job.name"])
          - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.deployment.name"]) where IsString(resource.attributes["k8s.deployment.name"])
          - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.daemonset.name"]) where IsString(resource.attributes["k8s.daemonset.name"])
          - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.cronjob.name"]) where IsString(resource.attributes["k8s.cronjob.name"])
          - set(resource.attributes["k8s.workload.kind"], "statefulset") where IsString(resource.attributes["k8s.statefulset.name"])
          - set(resource.attributes["k8s.workload.kind"], "replicaset") where IsString(resource.attributes["k8s.replicaset.name"])
          - set(resource.attributes["k8s.workload.kind"], "job") where IsString(resource.attributes["k8s.job.name"])
          - set(resource.attributes["k8s.workload.kind"], "deployment") where IsString(resource.attributes["k8s.deployment.name"])
          - set(resource.attributes["k8s.workload.kind"], "daemonset") where IsString(resource.attributes["k8s.daemonset.name"])
          - set(resource.attributes["k8s.workload.kind"], "cronjob") where IsString(resource.attributes["k8s.cronjob.name"])
          - set(resource.attributes["dt.kubernetes.workload.name"], resource.attributes["k8s.workload.name"])
          - set(resource.attributes["dt.kubernetes.workload.kind"], resource.attributes["k8s.workload.kind"])
          - delete_key(resource.attributes, "k8s.statefulset.name")
          - delete_key(resource.attributes, "k8s.replicaset.name")
          - delete_key(resource.attributes, "k8s.job.name")
          - delete_key(resource.attributes, "k8s.deployment.name")
          - delete_key(resource.attributes, "k8s.daemonset.name")
          - delete_key(resource.attributes, "k8s.cronjob.name")
        - statements:
          - set(resource.attributes["k8s.pod.ip"], resource.attributes["ip"]) where resource.attributes["k8s.pod.ip"] == nil
        output:
            otlp:
                endpoint:
                    valueFrom:
                        secretKeyRef:
                            name: dynakube
                            namespace: ${DYNATRACE_NS}
                            key: apiurl
                path: v2/otlp/v1/traces
                headers:
                    - name: Authorization
                      prefix: Api-Token
                      valueFrom:
                          secretKeyRef:
                              name: dynakube
                              namespace: ${DYNATRACE_NS}
                              key: dataIngestToken
                protocol: http
    EOF
    ```

1. Deploy the Istio Telemetry resource, (see also [Traces Istio Support](./../../collecting-traces/istio-support.md)):

    ```bash
    kubectl apply -n istio-system -f - <<EOF
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: tracing-default
    spec:
      tracing:
      - providers:
        - name: "kyma-traces"
        randomSamplingPercentage: 1.0
    EOF
    ```

    The default **randomSamplingPercentage** is set to `1.0`, meaning it samples 1% of all requests. To change the sampling rate, adjust it as needed, up to 100 percent.

    > [!WARNING]
    > Be cautious when you configure the **randomSamplingPercentage**:
    > - Could cause high volume of traces.
    > - The Kyma trace gateway component does not scale automatically.

1. To find traces from your Kyma cluster in the Dynatrace UI, go to **Applications & Microservices** > **Distributed traces**.

### Ingest Metrics

To start ingesting custom and Istio metrics, deploy a MetricPipeline. The configuration of this pipeline depends on the aggregation temporality of your metrics.

> [!NOTE]
> The Dynatrace OpenTelemetry (OTLP) ingest API only accepts metrics with **delta** [aggregation temporality](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/otlp-api/ingest-otlp-metrics/about-metrics-ingest#aggregation-temporality). By contrast, many tools, including the OpenTelemetry SDK and the MetricPipeline `istio` and `prometheus` input, produce metrics with **cumulative** aggregation temporality by default. If your metrics are cumulative, you must use the Dynatrace OTel Collector, which transforms them to delta before sending them to Dynatrace.

Depending on your metrics source and temporality, choose one of the following methods:

- Ingest cumulative metrics using the Dynatrace OTel Collector for transformation. This solution is recommended as it often cumulative metrics cannot be avoided and it will provide the most flexibility. However, it will increase the number of additional components processing the data in the cluster (OTel Collector, ActiveGate) leading to increased resource consumption and increased chance of lossing data.

  1. Deploy the [MetricPipeline](./../../collecting-metrics/README.md) that ships to the Dynatrace OTel Collector:

        ```bash
        cat <<EOF | kubectl apply -f -
        apiVersion: telemetry.kyma-project.io/v1alpha1
        kind: MetricPipeline
        metadata:
            name: dynatrace
        spec:
            input:
                istio:
                    enabled: true
                prometheus:
                    enabled: true
            output:
                otlp:
                    endpoint:
                        value: http://dynakube-telemetry-ingest.${DYNATRACE_NS}:4317
        EOF
        ```

- If your application pushes OTLP metrics in delta temporality, and you don't use the MetricPipeline's `istio` or `prometheus` inputs, push the metrics directly to Dynatrace. Shipping data directly to the Dynatrace backend prevents unnecessary processing by additional components.

  To use this setup, you must explicitly enable the "delta" aggregation temporality as preferred temporality in your applications. You cannot enable additional inputs for the MetricPipeline because these produce metrics with "cumulative" temporality.

  1. Deploy the [MetricPipeline](./../../collecting-metrics/README.md):

        ```bash
        cat <<EOF | kubectl apply -f -
        apiVersion: telemetry.kyma-project.io/v1alpha1
        kind: MetricPipeline
        metadata:
            name: dynatrace
        spec:
          transform:
            - statements:
              - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.statefulset.name"]) where IsString(resource.attributes["k8s.statefulset.name"])
              - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.replicaset.name"]) where IsString(resource.attributes["k8s.replicaset.name"])
              - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.job.name"]) where IsString(resource.attributes["k8s.job.name"])
              - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.deployment.name"]) where IsString(resource.attributes["k8s.deployment.name"])
              - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.daemonset.name"]) where IsString(resource.attributes["k8s.daemonset.name"])
              - set(resource.attributes["k8s.workload.name"], resource.attributes["k8s.cronjob.name"]) where IsString(resource.attributes["k8s.cronjob.name"])
              - set(resource.attributes["k8s.workload.kind"], "statefulset") where IsString(resource.attributes["k8s.statefulset.name"])
              - set(resource.attributes["k8s.workload.kind"], "replicaset") where IsString(resource.attributes["k8s.replicaset.name"])
              - set(resource.attributes["k8s.workload.kind"], "job") where IsString(resource.attributes["k8s.job.name"])
              - set(resource.attributes["k8s.workload.kind"], "deployment") where IsString(resource.attributes["k8s.deployment.name"])
              - set(resource.attributes["k8s.workload.kind"], "daemonset") where IsString(resource.attributes["k8s.daemonset.name"])
              - set(resource.attributes["k8s.workload.kind"], "cronjob") where IsString(resource.attributes["k8s.cronjob.name"])
              - set(resource.attributes["dt.kubernetes.workload.name"], resource.attributes["k8s.workload.name"])
              - set(resource.attributes["dt.kubernetes.workload.kind"], resource.attributes["k8s.workload.kind"])
              - delete_key(resource.attributes, "k8s.statefulset.name")
              - delete_key(resource.attributes, "k8s.replicaset.name")
              - delete_key(resource.attributes, "k8s.job.name")
              - delete_key(resource.attributes, "k8s.deployment.name")
              - delete_key(resource.attributes, "k8s.daemonset.name")
              - delete_key(resource.attributes, "k8s.cronjob.name")
            - statements:
              - set(resource.attributes["k8s.pod.ip"], resource.attributes["ip"]) where resource.attributes["k8s.pod.ip"] == nil
            output:
                otlp:
                    endpoint:
                        valueFrom:
                            secretKeyRef:
                                name: dynakube
                                namespace: ${DYNATRACE_NS}
                                key: apiurl
                    path: v2/otlp/v1/metrics
                    headers:
                        - name: Authorization
                          prefix: Api-Token
                          valueFrom:
                            secretKeyRef:
                              name: dynakube
                              namespace: ${DYNATRACE_NS}
                              key: dataIngestToken
                    protocol: http
        EOF
        ```

  1. Start pushing metrics to the metric gateway using [delta aggregation temporality.](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/otlp-api/ingest-otlp-metrics/about-metrics-ingest#aggregation-temporality)

  1. To find metrics from your Kyma cluster in the Dynatrace UI, go to **Observe & Explore** > **Metrics**.

## Set Up Kyma Dashboard Integration

For easier access from the Kyma dashboard, add links to new navigation under **Dynatrace**.

1. Apply the ConfigMaps:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/user/integration/dynatrace/kyma-dashboard-configmap.yaml
    ```

2. If your Secret has a different name or namespace, then download the file first and adjust the namespace and name accordingly in the 'dataSources' section of the file.

## Use Dynatrace Dashboards

1. To see the health of the Kyma Telemetry module and its related pipelines, import the file [Telemetry Module Status](./telemetry-resource-metrics.json) as a Dynatrace dashboard. For details, see [Importing Dashboards](https://docs.dynatrace.com/docs/analyze-explore-automate/dashboards-classic/dashboards/dashboard-json#import-dashboard).

2. Add the following custom resource attributes to the allow list of OpenTelemetry metrics resource attributes:
   - `k8s.resource.name`
   - `k8s.resource.group`
   - `k8s.resource.kind`
   - `k8s.resource.version`

   For details about adding attributes to the allow list, see [Configure resource and scope attributes to be added as dimensions](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/otlp-api/ingest-otlp-metrics/configure-otlp-metrics#allow-list).

## Use Dynatrace Alerts

To send alerts about the Kyma Telemetry module status to your preferred backend system, create Dynatrace alerts based on certain metric events:

1. To define how and when alerts are triggered, create a problem alerting profile. For details, see [Create an alerting profile](https://docs.dynatrace.com/docs/analyze-explore-automate/notifications-and-alerting/alerting-profiles#create-an-alerting-profile).
2. To push alerts to your backend system, set up problem notifications in Dynatrace. For details, see [Problem notifications](https://docs.dynatrace.com/docs/analyze-explore-automate/notifications-and-alerting/problem-notifications).
3. Create a metric event with a metric selector or a metric key that reflects the event you want to monitor. For details, see [Metric events](https://docs.dynatrace.com/docs/discover-dynatrace/platform/davis-ai/anomaly-detection/set-up-a-customized-anomaly-detector/how-to-set-up/metric-events).
   For example, trigger an alert when the Kyma Telemetry module enters a non-ready state:

     ```text
     kyma.resource.status.state:filter(not(eq("state","Ready")))
     ```

4. To target the metric event you just created, add a custom event filter in your alerting profile. For details, see [event filters](https://docs.dynatrace.com/docs/analyze-explore-automate/notifications-and-alerting/alerting-profiles#event-filters).
5. To test the integration, trigger the metric event and confirm that the target system receives the alert.
