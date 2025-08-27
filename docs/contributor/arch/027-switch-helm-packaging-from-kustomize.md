---
title: Switch Kustomize Packaging to Helm
status: Accepted
date: 2025-08-21
---

# 26. Switch Kustomize Packaging to Helm

## Context

We often need templating capabilities to deploy the module across multiple environments. While Kustomize was initially chosen due to its integration with kubebuilder (which generates artifacts using Kustomize), its role in our setup has become minimal. At this point, Kustomize is only used for CRD generation.

Because these CRDs are unaffected by the deployment layer, we now have the opportunity to switch the main deployment mechanism to Helm, which is the de-facto standard for Kubernetes packaging and offers significantly more powerful templating and configuration features. Therefore, the proposal is to convert the existing `config/` folder into a Helm chart, and adapt the deployment and testing tooling accordingly.

## Proposal

The proposal is to switch the deployment packaging from Kustomize to Helm. This involves the following steps:
1. Convert the existing `config/` folder into a Helm chart, maintaining the same structure and functionality.
2. Adapt the deployment scripts and CI/CD pipelines to use Helm for deploying the module.
3. Update the documentation to reflect the changes in deployment procedures.
4. Ensure that all existing functionality is preserved and that the new Helm-based deployment is tested thoroughly.
5. Maintain the CRD generation using ControllerGen, as it is not affected by the deployment layer.

### CRD Manifests
The Helm CRDs are treated as a special kind of object. They are installed before the rest of the chart. However, they have some limitations:
- They are not templated, so they must be plain YAML documents.
- They are not deleted when the chart is uninstalled, so they must be managed separately.
- They are not upgraded when the chart is upgraded, so they must be managed separately.

As we have two different sets of CRDs (one regular release and experimental release), the Helm CRDs mechanism is not suitable for our use case. Instead, we will keep the CRD manifests in a separate folder as subcharts, and use Helm values to include or exclude them based on the deployment type (regular or experimental).

### Chart and Application Versioning

Helm distinguishes between the chart version (`version`) and the application version (`appVersion`):

- Chart Version (`version`)
    - Refers to the version of the Helm chart itself (templates, defaults, dependencies).
    - Must follow [Semantic Versioning](https://semver.org/).
    - Should be incremented whenever chart contents change:
        - Patch for backwards-compatible fixes (e.g., adjusting default values).
        - Minor for backwards-compatible feature additions.
        - Major for breaking changes (e.g., renamed resources or values).
    - Used by Helm to track and resolve chart upgrades.

- Application Version (`appVersion`)
    - Refers to the version of the deployed application (typically the Docker image tag).
    - Purely informational from Helm’s perspective but helps humans trace which application version a given chart release deploys.
    - Should be kept in sync with the released application version.

#### Best Practices
- Treat chart versioning as independent of application versioning.
- Use CI/CD automation to bump `appVersion` whenever a new application image is released.
- Do not overwrite existing chart versions in the chart repository; each release is immutable.
- Release artifacts will carry both versions, e.g.:
    - `version: 0.1.2`
    - `appVersion: 1.47.0`
  
## Decision
- The `config/` folder will be converted into a Helm chart, in a new folder `helm/telemetry-module` in the root of the repository.
- The CRDs will be maintained in a separate folder as subcharts (regular amd experimental), and included or excluded based on the deployment type.
- The CRDs generation will continue to use ControllerGen.
- The deployment scripts and CI/CD pipelines will be updated to use Helm.
- The release artifact will be a packaged as plain YAML files as before, but generated using `helm template`.
- Chart versioning will follow semantic versioning and be updated manually when chart changes are introduced.
- Application versioning (`appVersion`) will be updated whenever a new application image is released.
- Both chart and application versions will be published together in the release artifacts to provide clear traceability between packaging logic and deployed application versions.