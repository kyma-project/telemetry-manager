# Integrate Kyma with AWS CloudWatch

## Overview

The Kyma Telemetry module supports you in integrating with observability backends in a convenient way. The following example outlines how to integrate with [AWS CloudWatch](https://aws.amazon.com/cloudwatch) as a backend. Because CloudWatch does not support OTLP ingestion natively, you must deploy the [AWS Distro for OpenTelemetry](https://aws-otel.github.io) additionally.

![overview](../assets/cloudwatch.svg)

## Prerequisistes

- Kyma as the target deployment environment
- AWS account with permissions to create new users and security policies

## Installation

### Preparation

1. Export your Namespace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export KYMA_NS="{NAMESPACE}"
    ```
1. If you haven't created a Namespace yet, do it now:
    ```bash
    kubectl create namespace $KYMA_NS
    ```

### Create AWS IAM User

Create an IAM user and assign it to the specific IAM policies that are needed to let the AWS distro communicate with the AWS services.

First, create the IAM policy for pushing your metrics:
1. In your AWS account, search for the IAM service, go to **Policies** > **Create policy**, and remove the `Popular services` flag.
1. Select the **CloudWatch** service.
1. Select the actions **GetMetricData**, **PutMetricData**, **ListMetrics** and click **Next**.
1. Enter the policy name and click **Create policy**.

Next, create the policy for CloudWatch Logs:
1. In your AWS account, search for the IAM service, go to **Policies** > **Create policy**, and remove the `Popular services` flag.
1. Select the `CloudWatch Logs` service.
1. Select the actions **CreateLogGroup**, **CreateLogStream**, **PutLogEvents** and click **Next**.
1. Specify the resource ARN for the selected actions.
1. Enter the policy name and click `Create policy`.

After creating the IAM Policies, create an IAM user:
1. In your AWS account, go to **Users** > **Add user**.
1. Enter the user name and click **Next**.
1. Select **Attach policies directly**.
1. Select the two policies you created previously, as well as the policy `AWSXrayWriteOnlyAccess`.
1. Click **Next** > **Create User**.
1. Open the new user.
1. Under **Security credentials**, click **Create access key**.
1. Select **Application running outside AWS** and then click **Next**.
1. Describe the purpose of this access key and click **Create access key**.
1. Copy and save the access key and the Secret access key.

### Create a secret with AWS Credentials

To connect the AWS Distro to the AWS services, put the credentials of the created IAM user into the Kyma cluster.

1. Create the Secret with kubectl. In the following command, replace the `{ACCESS_KEY}` and `{SECRET_ACCESS_KEY}` to your access keys, and `{AWS_REGION}` with the AWS region you want to use:
    ```bash
    kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID={ACCESS_KEY} --from-literal=AWS_SECRET_ACCESS_KEY={SECRET_ACCESS_KEY} --from-literal=AWS_REGION={AWS_REGION}
    ```

### Deploy the AWS Distro

After creating a Secret and configuring the required users in AWS, deploy the AWS Distro. It is a specific distribution of an OTel collector, which converts and dispatches the OTLP-based metrics and trace data in the cluster to the AWS-specific format and protocol.

1. Deploy the AWS Distro:
    ```bash
    kubectl -n $KYMA_NS apply -f ./resources/aws-otel.yaml
    ```

### Set up Kyma Telemetry

Use the Kyma Telemetry module to enable ingestion of the signals from your workloads:

1. Enable a LogPipeline that ships container logs of all workloads directly to the AWS X-Ray service. Use the same Secret as for the AWS Distro, bypassing the AWS Distro. Replace the `{NAMESPACE}` placeholder in the following command and run it:
    ```bash
    kubectl apply -f ./resources/logpipeline.yaml
    ```
   > **NOTE:** For now, the logging integration is inconsistent with the metrics and tracing integration. It will be updated to the common approach as soon as the AWS Distro supports logs, and the Kyma logging module uses OTLP.
1. Enable a TracePipeline in the cluster so that all components have a well-defined OTLP-based push URL in the cluster to send trace data to. Replace the `{NAMESPACE}` placeholder in the following command and run it:
    ```bash
    kubectl apply -f ./resources/tracepipeline.yaml
    ```
1. Enable a MetricPipeline in the cluster so that all components have a well-defined OTLP-based push URL in the cluster to send metric data to. Also, the MetricPipeline activates annotation-based metric scraping for workloads. Replace the `{NAMESPACE}` placeholder in the following command and run it:
    ```bash
    kubectl apply -f ./resources/metricpipeline.yaml
    ```

## Verify the results by deploying sample apps

To verify the results of CloudWatch and X-Ray, deploy sample applications for each service.

### Verify CloudWatch traces, logs, and metrics arrival

The sample app that generates traces in the following example is documented by AWS in their documentation: [Deploy a sample application to test the AWS Distro for OpenTelemetry Collector](https://docs.aws.amazon.com/eks/latest/userguide/sample-app.html).
1. Deploy the traffic generator app:
    ```bash
    kubectl apply -n ${KYMA_NS} -f ./sample-app/traffic-generator.yaml
    ```
1. Deploy an example app:
    ```bash
    kubectl apply -n ${KYMA_NS} -f ./sample-app/deployment.yaml
    ```
1. To access the application, port-forward it:
    ```bash
    kubectl -n ${KYMA_NS} port-forward svc/sample-app 4567
    ```
1. Make some requests to the application, for example, `localhost:4567` or `localhost:4567/outgoing-http-call`.
1. Go to **AWS X-Ray** > **Traces**.
1. To verify the logs, go to **AWS CloudWatch** > **Log groups** and select your cluster. Now, you can open `aws-integration.sample-app-*` and view the logs of your application.
1. To verify metrics, go to **All metrics** and open the `aws-integration/otel-collector`.

### Create the dashboard to observe incoming metrics and logs

1. Go to **Dashboards** > **Create dashboard** and enter the name.
1. Select the widget type and click **Next**.
1. Select what you want to observe, either metrics or logs, and click **Next**.
1. Decide which metrics or logs to include and click **Create widget**.

## AWS OTEL Collector and TraceId specifics

Currently, there is no mechanism to convert TraceId from W3C context into the AWS TraceId format. Because of that, your application should emit traces with IDs of the format compatible with AWS TraceId. To do that, you can use one of the available ADOT(AWS Distro for OpenTelemetry) SDKs:
* [Go](https://aws-otel.github.io/docs/getting-started/go-sdk)
* [Java](https://aws-otel.github.io/docs/getting-started/java-sdk)
* [JavaScript](https://aws-otel.github.io/docs/getting-started/javascript-sdk)
* [.NET](https://aws-otel.github.io/docs/getting-started/dotnet-sdk)
* [Python](https://aws-otel.github.io/docs/getting-started/python-sdk)
