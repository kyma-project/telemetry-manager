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
- After successful tests, the release workflow builds a new image from the same Dockerfile and source code, tags it with the release version and pushes it to the production registry.

**Current Challenge:**
Both images currently receive different digests due to different build environments and timestamps, despite having identical content and application binary. This behavior is not acceptable for auditable release automation.

**Resolution:**
We must implement deterministic Docker builds. An image built from a PR must have the same digest as the final release image built from the same source code. This makes the build process auditable.


## Implementing Auditable Release Automation

### Download and Store Test Reports

The test reports from unit tests, E2E tests, Gardener tests, and upgrade tests must be downloaded from the respective GitHub Actions workflows and stored as artifacts for audit purposes. For this purpose, we create a new reusable workflow that downloads and stores test reports from the workflow runs.
After the test jobs complete, the workflow uploads the test execution results to the specified GCP bucket (see [Archive Test Logs for 12-Month Auditing via Release Assets #8419](https://github.tools.sap/kyma/backlog/issues/8419).

### Deterministic Docker Builds
To achieve deterministic Docker builds and ensure audit compliance, we considered the following strategies:

- **Reproducible Build Tools**: Use tools and techniques that support reproducible builds, such as Docker's BuildKit with deterministic settings. This approach requires implementation support from the `Image-Builder` team and is not currently available.
- **Copy Release PR Image**: Modify the release process to copy the Docker image built during the PR directly to the production registry with the release tag. This approach ensures that the exact same image used for testing is released, maintaining identical digests.

**Recommended Approach:**

Because reproducible build tools are not yet available, the recommended approach is to run all tests (unit, E2E, Gardener, and upgrade) in the release branch using the release image before the final release publication.

This approach provides:
- Solid image digests proof for testing and release (audit compliance)
- Comprehensive test coverage in the release context
- Clear traceability for release audits

![Release Workflow](./../assets/auditable-release-final.drawio.svg)


### Module Release Workflow

After entering the release version and channel, the release master triggers a dedicated GitHub workflow to publish the module release. The workflow then creates the module configuration, assigns the release channel, and opens pull requests to update the \experimental`, `fast`, and `regular` channels.`

![Module Release Workflow](./../assets/auditable-release-module-release.drawio.svg)


### Management Plane Chart Update Workflow

After specifying the release version and a target chart, the release master triggers a dedicated GitHub workflow to bump Management Plane Charts. This workflow automates the release of charts such as \chart/telemetry` and `chart/runtime-monitoring-operator`.`

![Management Plane Chart Release Workflow](./../assets/auditable-release-mpc-release.drawio.svg)

## Release Workflow Steps

1. The Project Master closes the current development milestone to mark the end of the development phase for the release.

2. The Release Master triggers the release workflow by entering the release version and the OpenTelemetry Collector Components (OCC) version.

3. The system evaluates if the release is a patch or a new minor version: 
   - For a new minor version, it creates a dedicated `release-x.y` branch.
   - For a patch, it skips release branch creation.
4. The system commits the version bump and creates a release tag.
**Source Control Action**: Push the committed version-bumped artifacts and the release tag to the release branch. This officially marks the release version in the repository.
6. The system builds the release Docker image and runs unit tests in parallel.

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
By retaining test reports and ensuring Docker images are reproducible, we create a transparent and reliable release process that meets audit requirements.

The proposed strategies for deterministic builds and the structured release workflow provide a clear path to achieving this goal.

Currently, the recommended approach is to run all tests in the release branch using the release Docker image, ensuring that the same image is used for both testing and release, thus maintaining identical digests and providing a clear audit trail.

We can skip the PR tests and run them only in the release branch, so we can automate the release process without waiting for the PR tests to complete, and still ensure that the released image is tested and has the same digest as the one built in the PR.
A new GitHub Action can be implemented to trigger the release branch workflow once the release master enters the release version and OpenTelemetry Collector Components version for the release.

The release artifacts and GitHub release will be created once the release tests are successful and the release report is uploaded to the GCP bucket for audit retention.