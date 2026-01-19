# Samples

## Prerequisite

You have the Telemetry module in your cluster. See [Installation](./../docs/contributor/installation.md).

## Context

The samples are self-contained and include their own backends and data producers. You can apply a single sample or install all at once by applying the entire folder.

## Procedure

1. Apply all samples.

   ```bash
   kubectl apply -f .
   ```

1. Assure that the module is running healthily by checking the status of the Telemetry resource and the conditions of the pipeline resources.

   ```bash
   kubectl get kyma-telemetry
   ```

1. Delete the samples.

   ```bash
   kubectl delete -f .
   ```
