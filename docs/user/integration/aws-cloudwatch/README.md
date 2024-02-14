# Integrate Kyma with Amazon CloudWatch and AWS X-Ray

## Overview

| Category|                       |
| - |-----------------------|
| Signal types | traces, logs, metrics |
| Backend type | third-party remote    |
| OTLP-native | no                    |

Learn how to use [Amazon CloudWatch](https://aws.amazon.com/cloudwatch) and [AWS X-Ray](https://aws.amazon.com/xray/) as backends for the Kyma Telemetry module.

Fluent Bit ingests logs directly into CloudWatch using the [Amazon CloudWatch output plugin](https://docs.fluentbit.io/manual/pipeline/outputs/cloudwatch). Because CloudWatch and X-Ray do not support OTLP ingestion natively, the Metric Gateway and Trace Gateway must first ingest the OTLP Metrics and OTLP Traces into the [AWS Distro for OpenTelemetry](https://aws-otel.github.io). Then, the AWS Distro converts the OTLP Metrics and OTLP Traces to the format required by CloudWatch and X-Ray respectively and ingests the metrics into CloudWatch and traces into X-Ray.

![overview](../assets/cloudwatch.drawio.svg)

## Table of Content

- [Integrate Kyma with Amazon CloudWatch and AWS X-Ray](#integrate-kyma-with-amazon-cloudwatch-and-aws-x-ray)
  - [Overview](#overview)
  - [Table of Content](#table-of-content)
  - [Prerequisites](#prerequisites)
  - [Prepare the Namespace](#prepare-the-namespace)
  - [Set Up AWS Credentials](#set-up-aws-credentials)
    - [Create AWS IAM User](#create-aws-iam-user)
    - [Create a Secret with AWS Credentials](#create-a-secret-with-aws-credentials)
  - [Deploy the AWS Distro](#deploy-the-aws-distro)
  - [Set Up Kyma Pipelines](#set-up-kyma-pipelines)
  - [Verify the Results](#verify-the-results)

## Prerequisites

- Kyma as the target deployment environment
- The [Telemetry module](https://kyma-project.io/#/telemetry-manager/user/README) is [enabled](https://kyma-project.io/#/02-get-started/01-quick-install)
- [Kubectl version that is within one minor version (older or newer) of `kube-apiserver`](https://kubernetes.io/releases/version-skew-policy/#kubectl)
- AWS account with permissions to create new users and security policies


## Prepare the Namespace

1. Export your namespace as a variable. Replace the `{NAMESPACE}` placeholder in the following command and run it:

    ```bash
    export K8S_NAMESPACE="{NAMESPACE}"
    ```
1. If you haven't created a namespace yet, do it now:
    ```bash
    kubectl create namespace $K8S_NAMESPACE
    ```
## Set Up AWS Credentials

### Create AWS IAM User

1. In your AWS account, create an [IAM policy](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html) for the **CloudWatch Logs** service with the actions **CreateLogGroup**, **CreateLogStream**, **PutLogEvents**, and **PutRetentionPolicy**, and specify the resource [ARNs](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html) for the selected actions.
2. In your AWS account, create an [IAM user](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users.html) and attach the policy you just created, as well as the policy **AWSXrayWriteOnlyAccess**.
3. For the [IAM user](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users.html) you just created, create an access key for an application running outside AWS. Copy and Save the **access key** and **secret access key**; you need them to [Create a Secret with AWS Credentials](#create-a-secret-with-aws-credentials).


### Create a Secret with AWS Credentials

To connect the AWS Distro to the AWS services, create a secret containing the credentials of the created IAM user into the Kyma cluster. In the following command, replace `{ACCESS_KEY}` with your access key, `{SECRET_ACCESS_KEY}` with your secret access key, and `{AWS_REGION}` with the AWS region you want to use:
 
```bash
kubectl create secret generic aws-credentials -n $K8S_NAMESPACE --from-literal=AWS_ACCESS_KEY_ID={ACCESS_KEY} --from-literal=AWS_SECRET_ACCESS_KEY={SECRET_ACCESS_KEY} --from-literal=AWS_REGION={AWS_REGION}
 ```

## Deploy the AWS Distro

Deploy the AWS Distro, which is an AWS-supported distribution of an OTel collector. The [AWS X-Ray Tracing Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/awsxrayexporter) used in the collector converts OTLP traces to [AWS X-Ray Segment Documents](https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html) and then sends them directly to X-Ray. The [AWS CloudWatch EMF Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/awsemfexporter/README.md) used in the collector converts OTLP metrics to [AWS CloudWatch Embedded Metric Format(EMF)](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Specification.html) and then sends them directly to CloudWatch Logs.

> [!NOTE]
> The retention of these CloudWatch Logs is set to 7 days. You can change that to fit your needs by adjusting the `log_retention` value for the `awsemf` exporter in the [`aws-otel.yaml`](aws-otel.yaml) file.

 ```bash
kubectl -n $K8S_NAMESPACE apply -f aws-otel.yaml
 ```

## Set Up Kyma Pipelines

Use the Kyma Telemetry module to enable ingestion of the signals from your workloads:

1. Deploy a LogPipeline:
   > [!NOTE]
   > The retention of of the CloudWatch Logs is set to 7 days. You can change that to fit your needs by adjusting the `log_retention_days` value.

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
         region \${AWS_REGION}
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

Verify that the logs and metrics are exported to CloudWatch and that the traces are exported to X-Ray.

1. [Install the OpenTelemetry demo application](../opentelemetry-demo/README.md).
2. Go to `https://{AWS_REGION}.console.aws.amazon.com/cloudwatch`. Replace `{AWS_REGION}` with the region that you have chosen when [creating the secret with AWS credentials](#create-a-secret-with-aws-credentials).
3. To verify the traces: under **X-Ray traces**, go to **Traces**.
4. To verify the logs: under **Logs**, go to **Log groups** and select the log group of your cluster which has a name that follows the pattern `/logs/{CLUSTER_IDENTIFIER}`. Now, you can open the log stream you want and view the logs. The name of each log stream follows the pattern `{NAMESPACE}.{POD_NAME}.{CONTAINER_NAME}`.
5. To verify the metrics: under **Metrics**, go to **All metrics**.
