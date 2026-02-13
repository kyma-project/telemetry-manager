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
- The release preparation workflow builds a Docker image from a PR and pushes it to the Kyma dev registry with a unique PR tag and digest SHA. This image is used to run unit and E2E tests.
- After successful tests, the release workflow builds a new image from the  same Dockerfile and source code, tags it with the release version and pushes it to the production registry.

**Current Challenge:**
Both images currently receive different digests due to different build environments and timestamps, despite having identical content and application binary. This behavior is not acceptable for auditable release automation.

**Resolution:**
We must implement deterministic Docker builds. An image built from a PR must have the identical digest as the final release image built from the same source code. This makes the build process auditable.


## Implementing Auditable Release Automation

### Download and Store Test Reports

The test reports from unit tests, E2E tests, and Gardener tests have to be downloaded from the respective GitHub Actions workflows and stored as artifacts for audit purposes. For this pupose, a new re-usable workflow created to download and store test reports from workflow.
After test jobs are completed successfully, the test exwcution result will be uplodaed to the GCP bucket (see [Archive Test Logs for 12-Month Auditing via Release Assets #8419](https://github.tools.sap/kyma/backlog/issues/8419).

### Deterministic Docker Builds
To achieve deterministic Docker builds, we consider the following strategies:

- **Reproducible Build Tools**: Utilize tools and techniques that support reproducible builds, such as Docker's BuildKit option, which must be implemented by the `Image-Builder`.
- **Copy Release PR Image**: As an alternative to reproducible builds, we can modify the release process to copy the Docker image built during the release PR directly to the production registry with the release tag. This approach ensures that the same image used for testing is released, maintaining identical digests.

Both approaches ensure that the Docker image used for testing is the same as the one released, providing a clear audit trail and maintaining software integrity throughout the release process. 
However, the reproducible build approach is currently not available and have to be implemented by the `Image-Builder` team, therefore the recommended approach for now, repeat all test in the release branch with release Docker Image.

![Release Workflow](./../assets/auditable-release-final.drawio.svg)


### The Module Release Publishing Workflow

A separate GitHub workflow will be responsible for publishing the module release. This workflow will be triggered by the release master once the release version and release channel are entered. The workflow will handle the module config creation and channel assignment, and will be responsible for creating the release PRs for the `experimental`, `fast`, and `regular` channels.

![Module Release Workflow](./../assets/auditable-release-module-release.drawio.svg)


### The Management Plane Chart Release Workflow

A separate GitHub workflow will be responsible for bumping Managemene Plane Charts. This workflow will be triggered by the release master once the release version and the target chart. The workflow will handle the chart release process for `chart/telemetry` and `chart/runtime-monitoring-operator` Helm charts.

![Management Plane Chart Release Workflow](./../assets/auditable-release-mpc-release.drawio.drawio.svg)

## Release Workflow Step-by-Step Execution

**Project Master Action**: Close the current development milestone to signify the boundary between development and release phases. This marks the completion of all planned features for the current release.

**Release Master Action**: The release master initiates the release process by entering the release version and OpenTelemetry Collector Components (OCC) version for the release. This action triggers the release workflow and sets the stage for subsequent steps.

**System Decision Point**: The system evaluates whether this is a patch release or a new minor version release:
- **Patch Release**: If the release involves only bug fixes (patch release), skip release branch creation.
- **New Version Release**: If this is a new feature release (non-patch), create a dedicated release branch following the `release-x.y` naming convention (e.g., `release-1.0`).
- Commits these changes to the release branch
- Creates a release tag marking the version point
**Source Control Action**: Push the committed version-bumped artifacts and the release tag to the release branch. This officially marks the release version in the repository.
- Run Unit Tests and Release Image creation in parallel.

**Simultaneous Test Runs**: Three independent test suites are triggered in parallel against the release branch after Docker image created:
- **E2E (End-to-End) Tests**: Full system behavior verification
- **Gardener Integration Tests**: Compatibility validation with Gardener infrastructure platform
- **Upgrade Tests**: Validation of upgrade paths from previous versions to the new release

All tests execute against the same release Docker image to ensure reproducibility and consistency. Tests run in parallel to minimize total release cycle time.

**Completion Action**: Upon successful completion of all test suites, the system:
- Aggregates all test reports from the test workflows
- Downloads test execution logs and artifacts
- Uploads the complete test results to a pre-configured Google Cloud Storage (GCP) bucket

**Release Publication**:
- Create an official GitHub release entry for the release tag
- Attach release artifacts and binaries

**Release Bump PR Creation**: Upon successful release creation, automatically create release bump pull requests for `experimental` and `fast` channels.

## Conclusion

Implementing auditable release automation is essential for maintaining the integrity and compliance of the SAP BTP, Kyma runtime product. 
By ensuring that test reports are retained and that Docker images are reproducible, we can create a transparent and reliable release process that meets auditing requirements and enhances overall software quality.

The proposed strategies for deterministic Docker builds and the structured release workflow provide a clear path forward for achieving auditable release automation in the Kyma runtime product environment.

Currently, the recommended approach is to run all tests in the release branch using the release Docker image, ensuring that the same image is used for both testing and release, thus maintaining identical digests and providing a clear audit trail.

We can skip the PR tests and run them only in the release branch, so we can automate release process without waiting for the PR tests to complete, and still ensure that the released image is tested and has the same digest as the one built in the PR. 
A new GitHub Action, ca be implemeted to trigger the release branch workflow once the release master enter the release version and OpenTelemetry Collector Components version for the release.

The release artifacts and GitHub release will be created once the release tests are successful and the release report uploaded successfully to the GCP bucket for audit retention.