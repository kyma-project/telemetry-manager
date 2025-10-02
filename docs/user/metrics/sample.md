# Sample MetricPipeline

The sample MetricPipeline uses mTLS with a Secret, and ships data with GRPC. As input, it supports runtime metrics as well as Prometheus and Istio metrics (both with diagnostics). You can choose to drop push-based OTLP metrics, and you can filter metric collection from specified namespaces.

## MetricPipeline API

Adjust the following sample MetricPipeline to your needs:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    # Enable/Disable different metric sources
    runtime:
      enabled: true
      # Optionally filter runtime metrics by resource type
      # resources:
      #   pod:
      #     enabled: false
      #   container:
      #     enabled: false
      #   node:
      #     enabled: false
      #   volume:
      #     enabled: false
      #   daemonset:
      #     enabled: true
      #   deployment:
      #     enabled: true
      #   statefulset:
      #     enabled: true
      #   job:
      #     enabled: true
      # Optionally filter by namespace and containers
      # namespaces: # set to {} to include all namespaces including system namespaces
      #   include:  # Include specific namespaces
      #   - kyma-system  # This enables collection from all Kyma modules
      #   - namespace1
      #   - namespace2
      #   # OR
      #   exclude:  # Exclude specific namespaces
      #   - namespace3

    prometheus: 
      enabled: true
      # Enable scraping diagnostic metrics
      # diagnosticMetrics:
      #   enabled: true
      # Optionally filter by namespace and containers
      # namespaces: # set to {} to include all namespaces including system namespaces
      #   include:  # Include specific namespaces
      #   - kyma-system  # This enables collection from all Kyma modules
      #   - namespace1
      #   - namespace2
      #   # OR
      #   exclude:  # Exclude specific namespaces
      #   - namespace3
    istio:
      enabled: true
      # Enable scraping diagnostic metrics
      # diagnosticMetrics:
      #   enabled: true
      # Optionally filter by namespace and containers
      # namespaces: # set to {} to include all namespaces including system namespaces
      #   include:  # Include specific namespaces
      #   - kyma-system  # This enables collection from all Kyma modules
      #   - namespace1
      #   - namespace2
      #   # OR
      #   exclude:  # Exclude specific namespaces
      #   - namespace3

    otlp:
      # disabled: true # Uncomment to disable push-based OTLP metrics
      # Optionally filter by namespace and containers
      # namespaces: # set to {} to include all namespaces including system namespaces
      #   include:  # Include specific namespaces
      #   - kyma-system  # This enables collection from all Kyma modules
      #   - namespace1
      #   - namespace2
      #   # OR
      #   exclude:  # Exclude specific namespaces
      #   - namespace3

  output:
    otlp:
      # protocol: http # Uncomment to use HTTP protocol, defaults to gRPC
      endpoint:
        valueFrom: # Store endpoint in a Secret to have it colocated with the authentication details
          secretKeyRef:
            name: otlp-backend-credentials
            namespace: default # Replace with your namespace if needed
            key: endpoint
      # mTLS Authentication (using a Secret - recommended)
      tls:
        cert:
          valueFrom:
            secretKeyRef:
              name: otlp-backend-credentials
              namespace: default
              key: tls.crt
        key:
          valueFrom:
            secretKeyRef:
              name: otlp-backend-credentials
              namespace: default
              key: tls.key
```

## Secret

The referenced Secret must have the referenced name, be located in the referenced namespace, and contain the mapped key. See the following example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: otlp-backend-credentials
  namespace: default # Replace with your namespace if needed
stringData:
  endpoint: https://backend.example.com:4317 # Or 4318 for HTTP
  tls.key: |
    -----BEGIN CERTIFICATE-----
    ...
  
  tls.cert: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
```

## Create a pipeline

1. Create a YAML file that defines the pipeline resource.
   The file specifies the pipeline's kind (MetricPipeline) according to the signal type you want to collect. Within the spec, you define the input and output configurations. For details, see the sample CRD files for each pipeline
1. To deploy the pipeline, apply the YAML file to your Kubernetes cluster using the command kubectl apply -f <filename>.yaml.
1. After deployment, check that your pipeline is healthy using kubectl:

```bash
kubectl get metricpipeline
NAME      CONFIGURATION GENERATED   GATEWAY HEALTHY   AGENT HEALTHY   FLOW HEALTHY
backend   True                      True              True            True
```
