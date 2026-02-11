---
title: Auditable Release Automation
status: Proposed
date: 2026-01-20
---

# Auditable Release Automation

## Context

In the context of the SAP BTP, Kyma runtime product, ensuring a reliable and auditable release process is crucial for maintaining software quality and compliance. An automated release pipeline not only streamlines the deployment process but also provides a clear audit trail of changes, approvals, and deployments.

This document outlines the principles and practices for implementing auditable release automation within the Kyma runtime product environment.

## Principles of Auditable Release Automation

For auditing purposes, each release must provide successful test result artifacts. This includes:

- Test execution reports (unit and e2e tests)
- Docker images used for testing and release
- Gardener tests execution reports
- Upgrade tests execution reports
- Docker Image digest verification

### Test Execution Reports

Test execution reports should be downloaded for unit tests, end-to-end (E2E) tests, and Gardener Integration tests from corresponding GitHub Workflow executions and retained for audit purposes.

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

### Download and Store Test Reports

The test reports from unit tests, E2E tests, and Gardener tests should be downloaded from the respective GitHub Actions workflows and stored as artifacts for audit purposes. For this pupose, a new re-usable workflow created to download and store test reports based on workflow run ID and job name.
The new workflow can be called from the release PR workflow after test jobs are completed successfully and upload to the pre-configured GCP bucket for audit retention as desccribed [here](https://github.tools.sap/kyma/backlog/issues/8419).

### Deterministic Docker Builds
To achieve deterministic Docker builds, the following strategies can be employed:

- **Reproducible Build Tools**: Utilize tools and techniques that support reproducible builds, such as Docker's BuildKit option, which have to be implemented by the `Image-Builder`.
- **Copy Release PR Image**: As an alternative to reproducible builds, the release process can be modified to copy the Docker image built during the release PR directly to the production registry with the release tag. This approach ensures that the same image used for testing is released, maintaining identical digests.

Both approaches ensure that the Docker image used for testing is the same as the one released, providing a clear audit trail and maintaining software integrity throughout the release process. 
However, the reproducible build approach is currently not available and have to be implemented by the `Image-Builder` team, therefore the recommended approach for now, repeat all test in the release branch with release Docker Image.

![Release Workflow](./../assets/auditable-release-final.drawio.svg)

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

**Release Bump PR Creation**: Upon successful release creation, automatically create release bump pull requests across multiple channels and environments:

## Conclusion

Implementing auditable release automation is essential for maintaining the integrity and compliance of the SAP BTP, Kyma runtime product. 
By ensuring that test reports are retained and that Docker images are reproducible, we can create a transparent and reliable release process that meets auditing requirements and enhances overall software quality.

The proposed strategies for deterministic Docker builds and the structured release workflow provide a clear path forward for achieving auditable release automation in the Kyma runtime product environment.

Currently, the recommended approach is to run all tests in the release branch using the release Docker image, ensuring that the same image is used for both testing and release, thus maintaining identical digests and providing a clear audit trail.

We can skip the PR tests and run them only in the release branch, so we can automate release process without waiting for the PR tests to complete, and still ensure that the released image is tested and has the same digest as the one built in the PR. 
A new GitHub Action, ca be implemeted to trigger the release branch workflow once the release master enter the release version and OpenTelemetry Collector Components version for the release.

The release artifacts and GitHub release will be created once the release tests are successful and the release report uploaded successfully to the GCP bucket for audit retention.