# Sample LogPipeline

The sample LogPipeline uses mTLS with a Secret, and ships data with gRPC. You can choose another protocol and another authentication method. Optionally, filter by namespace and containers. Consider setting up Istio access logs.

## LogPipeline API

Adjust the following sample LogPipeline to your needs:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: LogPipeline
metadata:
  name: backend
spec:
  input:
    application:
      enabled: true
      # Optionally filter by namespace and containers
      # namespaces:
      #   include:  # Include specific namespaces
      #   - kyma-system  # This enables collection from all Kyma modules
      #   - namespace1
      #   - namespace2
      #   # OR
      #   exclude:  # Exclude specific namespaces
      #   - namespace3
      #.  # OR
      #.  system: false # Include all namespaces including system namespaces
      # containers:
      #   include:  # Include specific containers
      #   - container1
      #   - container2
      #   # OR
      #   exclude:  # Exclude specific containers
      #   - container3
      keepOriginalBody: true # Keep the original log message after JSON parsing

    otlp:
      disabled: false
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
   The file specifies the pipeline's kind (LogPipeline) according to the signal type you want to collect. Within the spec, you define the input and output configurations. For details, see the sample CRD files for each pipeline
1. To deploy the pipeline, apply the YAML file to your Kubernetes cluster using the command kubectl apply -f <filename>.yaml.
1. After deployment, check that your pipeline is healthy using kubectl:

    ```sh
    kubectl get logpipeline
    NAME      CONFIGURATION GENERATED   GATEWAY HEALTHY   AGENT HEALTHY   FLOW HEALTHY
    backend   True                      True              True            True
    ```
