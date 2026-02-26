---
title: Auditable Release Automation
status: Proposed
date: 2026-01-20
---

# Auditable Release Automation

## Context

To ensure the quality and compliance of the SAP BTP, Kyma runtime product, we must have a reliable and auditable release process. Our current process lacks a clear audit trail.

This document proposes a new, automated release process that produces the required audit artifacts for changes, approvals, and deployments.

## Audit Requirements

To pass an audit, the release process must produce the following artifacts:

- Test execution reports (unit and e2e tests)
- Docker images used for testing and release
- Gardener tests execution reports
- Upgrade tests execution reports
- Docker Image digest verification

### Test Execution Reports

The release workflow must download and retain the execution reports for all unit and end-to-end (E2E) tests from their corresponding GitHub workflow runs.

### Docker Images

The Docker image used for testing must be the exact same image that is used for the release. Our release process must produce reproducible and traceable images.

**Current Flow:**
Our current release pipeline builds two separate images:
- The release preparation workflow builds a Docker image from a PR, and pushes it to the Kyma dev registry with a unique PR tag and digest SHA. This image is used to run unit and E2E tests.
- After successful tests, the release workflow builds a new image from the same Dockerfile and source code, tags it with the release version and pushes it to the production registry.

**Current Challenge:**
Both images currently receive different digests due to different build environments and timestamps, despite having identical content and application binaries. This behavior is not acceptable for auditable release automation.

**Resolution:**
We must implement deterministic Docker builds. An image built from a PR must have the same digest as the final release image built from the same source code. This makes the build process auditable.


## Implementing Auditable Release Automation

### Download and Store Test Reports

The test reports from unit tests, E2E tests, Gardener tests, and upgrade tests must be downloaded from the respective GitHub Actions workflows and stored as artifacts for audit purposes. For this purpose, we create a new reusable workflow that downloads and stores test reports from the workflow runs.
After the test jobs complete, the workflow uploads the test execution results to the specified GCP bucket (see [Archive Test Logs for 12-Month Auditing via Release Assets #8419](https://github.tools.sap/kyma/backlog/issues/8419)).

### Deterministic Docker Builds
To achieve deterministic Docker builds and ensure audit compliance, we considered the following strategies:

- **Reproducible Build Tools**: Use tools and techniques that support reproducible builds, such as Docker's BuildKit with deterministic settings. This approach requires implementation support from the `Image-Builder` team and is not currently available.
- **Copy Release PR Image**: Modify the release process to copy the Docker image built during the PR directly to the production registry with the release tag. This approach ensures that the exact same image used for testing is released, maintaining identical digests.

### Release Workflow

Because reproducible build tools are not yet available, the recommended approach is to run all tests (unit, E2E, Gardener, and upgrade) in the release branch using the release image before the final release publication.

This approach provides the following benefits:
- Solid proof of image digests for testing and release (audit compliance)
- Comprehensive test coverage in the release context
- Clear traceability for release audits

![Release Workflow](./../assets/auditable-release-final.drawio.svg)

1. The Release Master closes the current development milestone to mark the end of the development phase for the release.

2. After entering the release version and the OpenTelemetry Collector Components (OCC) version, the Release Master triggers the release workflow.

3. The system evaluates if the release is a patch or a new minor version:
   - For a new minor version, it creates a dedicated `release-x.y` branch.
   - For a patch, it skips release branch creation.
4. The system commits the version bump and creates a release tag. This officially marks the release version in the repository.

5. The system builds the release Docker image and runs the following tests in parallel against the release branch:
   - **E2E (End-to-End) Tests**: Full system behavior verification
   - **Gardener Integration Tests**: Compatibility validation with Gardener infrastructure platform
   - **Upgrade Tests**: Validation of upgrade paths from previous versions to the new release

6. If all test suites complete successfully, the system aggregates all test reports, downloads test execution logs and artifacts, and uploads the complete test results to a preconfigured Google Cloud Storage (GCP) bucket.

7. The system creates an official GitHub release entry for the release tag and attaches release artifacts and binaries.

8. After the GitHub release is published, the system automatically creates pull requests to bump the module version in the experimental and fast channels.

### Module Release Workflow

After entering the release version and channel, the Release Master triggers a dedicated GitHub workflow to publish the module release. The workflow creates the module configuration, assigns the release channel, and opens pull requests to update the `experimental`, `fast`, and `regular` channels.

![Module Release Workflow](./../assets/auditable-release-module-release.drawio.svg)


### Management Plane Chart Update Workflow

After specifying the release version and a target chart, the Release Master triggers a dedicated GitHub workflow to bump Management Plane Charts. This workflow releases charts such as `chart/telemetry` and `chart/runtime-monitoring-operator`, and creates pull requests to update the chart versions and values in the Management Plane repository.

![Management Plane Chart Release Workflow](./../assets/auditable-release-mpc-release.drawio.svg)



## Conclusion

This document proposes an auditable release automation process that addresses the compliance requirements for the SAP BTP, Kyma runtime product by establishing a comprehensive audit trail for all release activities.

The proposed solution ensures auditability through the following mechanisms:

1. Single Source of Truth: By running all test suites (unit, E2E, Gardener, and upgrade) against a single release Docker image built from the release branch, we guarantee that the tested artifact is identical to the deployed artifact, with verifiable image digest matching.
2. Complete Traceability: All test execution reports are collected and archived to GCP storage with 12-month retention, providing a complete audit trail of the quality gates each release has passed.
3. Automated Governance: The release workflow enforces the required approval gates and milestone closures, while automatically generating release artifacts and propagating changes through the experimental, fast, and regular channels using pull requests.

This approach eliminates the gap where separate images were built for testing and production, preventing digest mismatches that compromise audit integrity. The parallel execution of test suites minimizes release cycle time while maintaining comprehensive coverage. The automated creation of module releases and management plane chart updates ensures consistency across the release ecosystem.

The release process will be implemented as a single GitHub Actions workflow triggered by the Release Master with release version and OCC version inputs. This workflow orchestrates all stages from branch creation through testing to final publication, ensuring that manual intervention is limited to authorization decisions, not mechanical execution steps.

By adopting this auditable release automation, we establish a repeatable, transparent, and compliant release process that satisfies audit requirements while improving release velocity and reliability.
