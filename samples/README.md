# Samples

## Overview

The files in this folder can be applied to a Kubernetes cluster having the telemetry module installed in order to see the module with most of the features in action. The samples are self-contained and bring own backends and data producers and can be installed all at once by applying the whole folder.

## Procedure

1. Apply the samples

```bash
kubectl apply -f .
```

1. Assure that the is running healthy by checking the status of the Telemetry resource and the conditions of the pipeline resources

```bash
kubectl get kyma-telemetry
```

1. Delete the samples

```bash
kubectl delete -f .
```
