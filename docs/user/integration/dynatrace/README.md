# Integrate with Dynatrace

## Overview
| Category| |
| - | - |
| Signal types | traces, metrics |
| Backend type | third-party remote |
| OTLP-native | yes, but dynatrace agent in parallel |

[Dynatrace](https://www.dynatrace.com) is an advanced Application Performance Management solution available as SaaS offering. It provides support for monitoring both the Kubernetes cluster itself and the workloads running on the cluster. To leverage the full power, the proprietary agent technology of Dynatrace must be installed. Still, leveraging the Kyma telemetry module, custom spans and Istio spans can be added as well as custom metrics to gain even more visibility. Get a brief introduction on how to setup Dynatrace and learn how to integrate the Kyma telemetry module.

[setup](./assets/integration.drawio.svg)

## Prerequisistes

- Kyma as the target deployment environment
- The [Telemetry module](https://kyma-project.io/#/telemetry-manager/user/README) is [enabled](https://kyma-project.io/#/02-get-started/08-install-uninstall-upgrade-kyma-module?id=install-uninstall-and-upgrade-kyma-with-a-module)
- Active Dynatrace environment with permissions to create new access tokens
- Helm 3.x if you want to deploy the [OpenTelemetry sample application](../opentelemetry-demo/README.md)

## Prepare the Namespace

1. Export your Namespace you want to use for Dynatrace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export DYNATRACE_NS="dynatrace"
    ```

1. If you haven't created a Namespace yet, do it now:

    ```bash
    kubectl create namespace $DYNATRACE_NS
    ```

## Dynatrace Setup

There are different ways to deploy Dynatrace on Kubernetes. All deployment options are based on the [Dynatrace Operator](https://github.com/Dynatrace/dynatrace-operator). By default, Dynatrace uses the Classic full-stack injection deployment option, but we highly recommend using the new Cloud-native full-stack injection for better stability. Check out the [deployment options on Kubernetes](https://www.dynatrace.com/support/help/setup-and-configuration/setup-on-container-platforms/kubernetes/get-started-with-kubernetes-monitoring/deployment-options-k8s) for more information.

1. Install Dynatrace

    Follow the instructions on how to [install the cloud-native fullstack](https://docs.dynatrace.com/docs/setup-and-configuration/setup-on-k8s/installation/cloud-native-fullstack) using the namespace created above. Assure that you configure the proper `apiUrl` of your environemnt in the dynakube resource 


1. Adjust the dynakube resource to exclude Kyma system namespaces

    Add the following snippet to your dynakube resource
    ```yaml
    namespaceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
        operator: NotIn
        values:
        - kyma-system
        - istio-system
    ```

1. Enable relevant kubernetes features in the environment

   Under `Settings` > `Cloud and virtualization` > `Kubernetes` enable relevant features, especially "Monitor annotated Prometheus exporters" will enable you to collect Istio metrics in an easy way

1. Enable Istio relevant features in the environment
   
    In the Dynatrace Hub, enable the "Istio Service Mesh" extension and annotate your services as outlined in the description

1. Collect custom metrics in prometheus format

    If you have workload exposing metrics in the prometheus format, you can [annotate the workload](https://docs.dynatrace.com/docs/platform-modules/infrastructure-monitoring/container-platform-monitoring/kubernetes-monitoring/monitor-prometheus-metrics). If the workload is having an istio-sidecar, you either need to weaken the mTLS setting for the metrics port by defining an [Istio PeerAuthentication](https://istio.io/latest/docs/reference/config/security/peer_authentication/#PeerAuthentication) or you exclude the port from interception by the istio proxy, by placing an `traffic.sidecar.istio.io/excludeInboundPorts` annotaion on your pod listing the metrics port.

1. Verify setup

    Now you should see data arriving in your environment and advanced kubernetes monitoring is possible. Also, Istio metrics will be available.

## Telemetry Module Setup

What is missing so far in the setup is the ingestion of custom and Istio span data. Custom metrics can be collected via the Dynatrace annotation approach. OTLP based metrics can be more easy collected by the telemetry module and pushed centrally the the environment.

### Create access token

To push custom metrics and spans to Dynatrace, an API Token is required:

1. In the Dynatrace navigation, go to the **Manage** > **Access tokens** > **Generate new token**.
1. Type the name you want to give to this token. If needed, set an expiration date.
1. Select the following scopes:
   - **Ingest metrics**
   - **Ingest OpenTelemetry traces**
1. Click **Generate token**.
1. Copy and save the generated token.

### Create Secret

1. To create a new secret containing your access token execute the following command. Replace the `{API_TOKEN}` placeholder with the previously created token and run it:

    ```bash
    kubectl -n $DYNATRACE_NS create secret generic dynatrace-token --from-literal="apiToken=Api-Token {API_TOKEN}"
    ```

### Ingest Traces

1. Deploy the Istio Telemetry resource:

    ```bash
    kubectl apply -n istio-system -f - <<EOF
    apiVersion: telemetry.istio.io/v1alpha1
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

    The default configuration has the **randomSamplingPercentage** property set to `1.0`, meaning it samples 1% of all requests. To change the sampling rate, adjust the property to the desired value, up to 100 percent.

    > **CAUTION:** Be cautious when you configure the **randomSamplingPercentage**:
    > - Traces might consume a significant storage volume in Cloud Logging Service.
    > - The Kyma trace collector component does not scale automatically.

1. Deploy the TracePipeline and replace the `{ENVIRONMENT_ID}` placeholder with the environment Id of your Dynatrace instance:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: TracePipeline
    metadata:
        name: dynatrace
    spec:
        output:
            otlp:
                endpoint:
                    value: https://{ENVIRONMENT_ID}.live.dynatrace.com/api/v2/otlp
                headers:
                    - name: Authorization
                      valueFrom:
                          secretKeyRef:
                              name: dynatrace-token
                              namespace: ${DYNATRACE_NS}
                              key: apiToken
                protocol: http
    EOF
    ```

1. To find traces from your Kyma cluster in the Dynatrace UI, go to **Applications & Microservices** > **Distributed traces**.

### Ingest Metrics (experimental)



1. Deploy the TracePipeline and replace the `{ENVIRONMENT_ID}` placeholder with the environment Id of Dynatrace SaaS:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: telemetry.kyma-project.io/v1alpha1
    kind: MetricPipeline
    metadata:
        name: dynatrace
    spec:
        output:
            otlp:
                endpoint:
                    value: https://{ENVIRONMENT_ID}.live.dynatrace.com/api/v2/otlp
                headers:
                    - name: Authorization
                      valueFrom:
                          secretKeyRef:
                              name: dynatrace-token
                              namespace: ${DYNATRACE_NS}
                              key: apiToken
                protocol: http
    EOF
    ```
1. To find metrics from your Kyma cluster in the Dynatrace UI, go to **Observe & Explore** > **Metrics**.
