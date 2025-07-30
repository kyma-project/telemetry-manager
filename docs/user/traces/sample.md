# Sample TracePipeline

The sample TracePipeline uses Basis Authentication with a Secret, and ships data with gRPC. You can choose another protocol and another authentication method. Remember to activate Istio tracing.

## TracePipeline API

Adjust the following sample TracePipeline to your needs:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: backend
spec:
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
   The file specifies the pipeline's kind (MTracePipeline) according to the signal type you want to collect. Within the spec, you define the input and output configurations. For details, see the sample CRD files for each pipeline
1. To deploy the pipeline, apply the YAML file to your Kubernetes cluster using the command kubectl apply -f <filename>.yaml.
1. After deployment, check that your pipeline is healthy using kubectl:

```bash
kubectl get tracepipeline
NAME      CONFIGURATION GENERATED   GATEWAY HEALTHY   FLOW HEALTHY
backend   True                      True              True
```
