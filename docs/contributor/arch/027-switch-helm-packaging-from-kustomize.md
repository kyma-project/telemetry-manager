---
title: Switch Kustomize Packaging to Helm
status: Accepted
date: 2025-08-21
---

# 26. Switch Kustomize Packaging to Helm

## Context

For deploying the module across multiple environments, templating capabilities are frequently required. While Kustomize was initially chosen due to its integration with kubebuilder (which generates artifacts using Kustomize), its role in our setup has become minimal. At this point, Kustomize is only used for CRD generation.

Since these CRDs are unaffected by the deployment layer, we now have the opportunity to switch the main deployment mechanism to Helm, which is the de-facto standard for Kubernetes packaging and offers significantly more powerful templating and configuration features. The proposal is therefore to convert the existing `config/` folder into a Helm chart, and adapt the deployment and testing tooling accordingly.

## Proposal

The proposal is to switch the deployment packaging from Kustomize to Helm. This involves the following steps:
1. Convert the existing `config/` folder into a Helm chart, maintaining the same structure and functionality.
2. Adapt the deployment scripts and CI/CD pipelines to use Helm for deploying the module.
3. Update the documentation to reflect the changes in deployment procedures.
4. Ensure that all existing functionality is preserved and that the new Helm-based deployment is tested thoroughly.
5. Maintain the CRD generation using ControllerGen, as it is not affected by the deployment layer.

### CRD Manifests
The Helm CRDs are treated as a special kind of object. They are installed before the rest of the chart, however, they have some limitations:
- They are not templated, so they must be plain YAML documents.
- They are not deleted when the chart is uninstalled, so they must be managed separately.
- They are not upgraded when the chart is upgraded, so they must be managed separately.

As we have two different sets of CRDs (one regular release and experimental release), the Helm CRDs mechanism is not suitable for our use case. Instead, we will keep the CRD manifests in a separate folder as subcharts, and use Helm values to include or exclude them based on the deployment type (regular or experimental).


## Decision
- The `config/` folder will be converted into a Helm chart, in a new folder `helm/telemetry-module` in the root of the repository.
- The CRDs will be maintained in a separate folder as subcharts (regular amd experimental), and included or excluded based on the deployment type.
- The CRDs generation will continue to use ControllerGen.
- The deployment scripts and CI/CD pipelines will be updated to use Helm.
- The release artifact will be a packaged as plain YAML files as before, but generated using `helm template`.