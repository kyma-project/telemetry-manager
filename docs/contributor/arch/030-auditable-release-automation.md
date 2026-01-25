# Auditable Release Automation

## Context

In the context of the SAP BTP, Kyma runtime product, ensuring a reliable and auditable release process is crucial for maintaining software quality and compliance. An automated release pipeline not only streamlines the deployment process but also provides a clear audit trail of changes, approvals, and deployments.

This document outlines the principles and practices for implementing auditable release automation within the Kyma runtime product environment.

## Principles of Auditable Release Automation

For auditing purposes, each release must provide successful test result artifacts. This includes:

- Test execution reports (unit and e2e tests)
- Docker images used for testing and release
- Gardener tests execution reports
- Release manifest with audit metadata
- Image digest verification records

### Test Execution Reports

Test execution reports should be downloaded for both unit tests and end-to-end (E2E) tests from corresponding GitHub Workflow executions and retained for audit purposes.

### Docker Images

The Docker image used for testing and release should be reproducible and traceable. The current release pipeline builds two Docker images: first for testing and second for release.

**Current Flow:**
- First Docker image is built during release preparation PR and pushed to the Kyma dev registry with a unique PR tag and digest SHA. This image is used for running unit and E2E tests.
- After successful tests, the second image is built for release using the same Dockerfile and source code, but tagged with release version and pushed to the production registry.

**Current Challenge:**
Both images currently receive different digests due to different build environments and timestamps, despite having identical content and application binary. This behavior is not acceptable for auditable release automation.

**Resolution:**
Implement deterministic Docker builds to ensure PR and release images produce identical digests when built from the same source code.


## Implementing Auditable Release Automation

### 1. Release Workflow

![arch](./../assets/auditable-release.drawio.svg)