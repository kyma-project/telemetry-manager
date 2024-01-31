# Integrate Kyma with AWS CloudWatch

## Overview

The Kyma Telemetry module supports you in integrating with observability backends in a convenient way. The following example outlines how to integrate with [AWS CloudWatch](https://aws.amazon.com/cloudwatch) as a backend. Because CloudWatch does not support OTLP ingestion natively, you must deploy the [AWS Distro for OpenTelemetry](https://aws-otel.github.io) additionally.

![overview](../assets/cloudwatch.drawio.svg)

## Table of Content

- [Prerequisites](#prerequisites)
- [Preparation](#preparation)
- [Set Up AWS Credentials](#set-up-aws-credentials)
- [Deploy the AWS Distro](#deploy-the-aws-distro)
- [Set Up Kyma Pipelines](#set-up-kyma-pipelines)
- [Verify the Results](#verify-the-results)

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](https://kyma-project.io/#/telemetry-manager/user/README) is [enabled](https://kyma-project.io/#/02-get-started/01-quick-install)
- Kubectl version 1.22.x or higher
- AWS account with permissions to create new users and security policies


## Preparation

1. Export your Namespace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export K8S_NAMESPACE="{NAMESPACE}"
    ```
1. If you haven't created a Namespace yet, do it now:
    ```bash
    kubectl create namespace $K8S_NAMESPACE
    ```
## Set Up AWS Credentials

### Create AWS IAM User

Create an IAM user and assign to it the specific IAM policies that are needed to let the AWS Distro communicate with the AWS services.

First, create the IAM policy for **CloudWatch** service:
1. In your AWS account, search for **IAM**, go to **Policies** > **Create policy**.
1. Select the **CloudWatch** service.
1. Select the actions **GetMetricData**, **PutMetricData**, **ListMetrics** and click **Next**.
1. Enter the policy name and click **Create policy**.

Next, create the IAM policy for **CloudWatch Logs** service:
1. In your AWS account, search for **IAM**, go to **Policies** > **Create policy**.
1. Select the **CloudWatch Logs** service.
1. Select the actions **CreateLogGroup**, **CreateLogStream**, **PutLogEvents**, **PutRetentionPolicy** and click **Next**.
1. Specify the resource [ARNs](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html) for the selected actions.
1. Enter the policy name and click **Create policy**.

After creating the IAM Policies, create an IAM user:
1. In your AWS account, search for **IAM**, go to **Users** > **Create user**.
1. Enter the user name and click **Next**.
1. Select **Attach policies directly**.
1. Select the two policies you created previously, as well as the policy **AWSXrayWriteOnlyAccess**.
1. Click **Next** > **Create User**.
1. Open the new user.
1. Under **Security credentials**, click **Create access key**.
1. Select **Application running outside AWS** and click **Next**.
1. Describe the purpose of this access key and click **Create access key**.
1. Copy and save the access key and the secret access key.

### Create a Secret with AWS Credentials

To connect the AWS Distro to the AWS services, create a secret containing the credentials of the created IAM user into the Kyma cluster. In the following command, replace `{ACCESS_KEY}` with your access key, `{SECRET_ACCESS_KEY}` with your secret access key, and `{AWS_REGION}` with the AWS region you want to use:
 
```bash
 kubectl create secret generic aws-credentials -n $K8S_NAMESPACE --from-literal=AWS_ACCESS_KEY_ID={ACCESS_KEY} --from-literal=AWS_SECRET_ACCESS_KEY={SECRET_ACCESS_KEY} --from-literal=AWS_REGION={AWS_REGION}
 ```

## Deploy the AWS Distro

Deploy the AWS Distro which is a specific distribution of an OTel collector. It converts and dispatches the OTLP-based metrics and traces in the cluster to the AWS-specific format and protocol.

 ```bash
 kubectl -n $K8S_NAMESPACE apply -f aws-otel.yaml
 ```

## Set Up Kyma Pipelines

Use the Kyma Telemetry module to enable ingestion of the signals from your workloads:

1. Deploy a LogPipeline:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: telemetry.kyma-project.io/v1alpha1
   kind: LogPipeline
   metadata:
     name: aws-cloudwatch
   spec:
     output:
       custom: |
         Name cloudwatch_logs
         region \$AWS_REGION
         auto_create_group On
         log_group_template /logs/\$cluster_identifier
         log_group_name /logs/kyma-cluster         
         log_stream_template \$kubernetes['namespace_name'].\$kubernetes['pod_name'].\$kubernetes['container_name']
         log_stream_name from-kyma-cluster
         log_retention_days 7
     variables:
       - name: AWS_ACCESS_KEY_ID
         valueFrom:
           secretKeyRef:
             name: aws-credentials
             namespace: $K8S_NAMESPACE
             key: AWS_ACCESS_KEY_ID
       - name: AWS_SECRET_ACCESS_KEY
         valueFrom:
           secretKeyRef:
             name: aws-credentials
             namespace: $K8S_NAMESPACE
             key: AWS_SECRET_ACCESS_KEY
       - name: AWS_REGION
         valueFrom:
           secretKeyRef:
             name: aws-credentials
             namespace: $K8S_NAMESPACE
             key: AWS_REGION
   EOF
   ```
   > NOTE: For now, the logs integration is inconsistent with the metrics and traces integration. It will be updated to the common approach as soon as the AWS Distro supports logs and the Kyma Telemetry module uses OTLP logs.

2. Deploy a TracePipeline:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: telemetry.kyma-project.io/v1alpha1
   kind: TracePipeline
   metadata:
     name: aws-xray
   spec:
     output:
       otlp:
         endpoint:
           value: http://otel-collector.$K8S_NAMESPACE.svc.cluster.local:4317
   EOF
   ```

3. Deploy a MetricPipeline:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: telemetry.kyma-project.io/v1alpha1
   kind: MetricPipeline
   metadata:
     name: aws-cloudwatch
   spec:
     input:
       runtime:
         enabled: true
       istio:
         enabled: true
       prometheus:
         enabled: true
     output:
       otlp:
         endpoint:
           value: http://otel-collector.$K8S_NAMESPACE.svc.cluster.local:4317
   EOF
   ```

## Verify the Results

To verify the results of CloudWatch and X-Ray, deploy sample applications for each service.

### Verify CloudWatch traces, logs, and metrics arrival

The sample app that generates traces in the following example is documented by AWS in their documentation: [Deploy a sample application to test the AWS Distro for OpenTelemetry Collector](https://docs.aws.amazon.com/eks/latest/userguide/sample-app.html).
1. Deploy the traffic generator app:
    ```bash
    kubectl apply -n ${K8S_NAMESPACE} -f ./sample-app/traffic-generator.yaml
    ```
1. Deploy an example app:
    ```bash
    kubectl apply -n ${K8S_NAMESPACE} -f ./sample-app/deployment.yaml
    ```
1. To access the application, port-forward it:
    ```bash
    kubectl -n ${K8S_NAMESPACE} port-forward svc/sample-app 4567
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
