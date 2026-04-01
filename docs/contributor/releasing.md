# Release Process

This document describes the automated release process for Telemetry Manager using GitHub Actions workflows.

<!-- TOC -->
* [Overview](#overview)
* [Prerequisites](#prerequisites)
* [Release to Experimental and Fast Channel](#release-to-experimental-and-fast-channel)
  * [1. Prepare the Release](#1-prepare-the-release)
  * [2. Start the Release Workflow](#2-start-the-release-workflow)
  * [3. Automatic Validation](#3-automatic-validation)
  * [4. Automatic Branch Creation](#4-automatic-branch-creation)
  * [5. Review and Merge the Version Bump PR](#5-review-and-merge-the-version-bump-pr)
  * [6. Automatic Testing](#6-automatic-testing)
  * [7. Automatic Tag and Release Creation](#7-automatic-tag-and-release-creation)
  * [8. Automatic Module Releases](#8-automatic-module-releases)
  * [9. Verify the Release](#9-verify-the-release)
* [Release to the Regular Channel](#release-to-the-regular-channel)
* [Monitor Release Progress](#monitor-release-progress)
  * [Workflow Status](#workflow-status)
  * [Module Release Status](#module-release-status)
* [Troubleshooting](#troubleshooting)
  * [Milestone Validation Error](#milestone-validation-error)
  * [Docker Image Not Found](#docker-image-not-found)
  * [GitHub Tag Already Exists](#github-tag-already-exists)
  * [Version Bump PR Times Out](#version-bump-pr-times-out)
* [Related Workflows](#related-workflows)
<!-- TOC -->

## Overview

The release process uses GitHub Actions workflows to automate the following tasks:
- Version validation and milestone verification
- Release branch creation and version bumping
- Docker image builds and tests
- GitHub release creation
- Module manifest updates for multiple channels

## Prerequisites

Ensure you have the following permissions:

- Write access to the telemetry-manager repository
- Access to merge PRs on the release branch

## Release to Experimental and Fast Channel

### 1. Prepare the Release

Before running the release workflow, complete the following tasks:

1. **Milestone Verification**:
   - Close all issues in the [GitHub milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the version.
   - Close the milestone.
   - Create a new [GitHub milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the next version.

2. **Release Component Dependencies**: Release the following component dependencies to produce the required Docker images
   - [Build Directory Size Exporter Image](https://github.com/kyma-project/telemetry-manager/actions/workflows/build-directory-size-reporter-image.yml) - Produces image tags like `v20260302-12345678`
   - [Build Self Monitor Image](https://github.com/kyma-project/telemetry-manager/actions/workflows/build-self-monitor-image.yml) - Produces image tags like `v20260302-bbf32a3b`
   - [OpenTelemetry Collector Components Create Release](https://github.com/kyma-project/opentelemetry-collector-components/actions/workflows/create-release.yaml) - Version format: **`{OCC_VERSION}`**-**`{TELEMETRY_VERSION}`**, such as `0.100.0-1.2.3`

3. **Verify Docker Image Availability**: After the component releases complete, verify that all required Docker images are available in the registry:
   ```bash
   # Check OCC image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:**{OCC_VERSION}-{TELEMETRY_VERSION}**

   # Check directory-size-exporter image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:**{DIR_SIZE_TAG}**

   # Check self-monitor image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/tpi/telemetry-self-monitor:**{SELF_MONITOR_TAG}**
   ```

### 2. Start the Release Workflow

In the telemetry-manager repository, go to **Actions**, select [Telemetry Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml), and run the release workflow with the following inputs:


| Input                      | Description                                                                      | Example              |
|----------------------------|----------------------------------------------------------------------------------|----------------------|
| **version**                | Release version in X.Y.Z format                                                  | `1.2.3`              |
| **occ_image_version**      | OCC image version in X.Y.Z-A.B.C format                                          | `0.100.0-1.2.3`      |
| **self_monitor_image_tag** | Self-monitor image tag in vYYYYMMDD-HASH format                                  | `v20260302-bbf32a3b` |
| **dir_size_image_tag**     | Directory size exporter image tag in vYYYYMMDD-HASH format                       | `v20260302-12345678` |
| **dry_run**                | Test the release process without creating tags/releases                          |                      |
| **force**                  | Recreate existing release (use with caution)                                     |                      |
| **module_release**         | Trigger module release for experimental and fast channels after the main release |                      |

> [!CAUTION]
> Force mode deletes the existing release and tag before recreating them. Use it only when necessary and communicate with the team beforehand.

Consider using force mode for the following purposes:
- Fixing a broken release
- Updating release assets
- Correcting version metadata

### 3. Automatic Validation

The release workflow automatically validates the following conditions:

- The version format follows semantic versioning (`X.Y.Z`)
- The OCC version format matches the expected pattern (`X.Y.Z-A.B.C`)
- The image tag format matches the expected pattern (`vYYYYMMDD-HASH`)
- All required Docker images exist in the registry
- The milestone exists, is closed, and has no open issues
- No existing release or tag conflicts with the target version
- The release branch exists for a patch release, or a new branch is needed for a minor/major release

> [!NOTE]
> The tag conflict check is skipped if force mode is enabled.

If validation fails, the release workflow stops and reports the error.

### 4. Automatic Branch Creation

To determine the release type, the release workflow checks if a `release-X.Y` branch already exists and handles branch preparation:

- For a minor or major release: The `release-X.Y` branch does not exist, so the release workflow creates it from the `main` branch.
- For a patch release: The `release-X.Y` branch already exists, so the release workflow uses the existing branch.

### 5. Review and Merge the Version Bump PR

When the release branch is ready, the workflow prepares the version updates and creates a pull request (PR) for your review. The workflow then pauses and waits for you to merge this PR before it can proceed.

> [!WARNING]
> The workflow fails if you do not merge the PR within 120 minutes.

1. Review the PR. Use the checklist in the PR description to verify that version numbers are correct, generated files are up to date, and no unintended changes are present.
2. Merge the PR into the release branch.

The PR contains the following changes:

- Updated variables in the `.env` file:

  | Variable                           | New value                  |
  |------------------------------------|----------------------------|
  | `ENV_HELM_RELEASE_VERSION`         | `{VERSION}`                |
  | `ENV_MANAGER_IMAGE` tag            | `{VERSION}`                |
  | `ENV_OTEL_COLLECTOR_IMAGE` tag     | `{OCC_IMAGE_VERSION}`      |
  | `ENV_SELFMONITOR_IMAGE` tag        | `{SELF_MONITOR_IMAGE_TAG}` |
  | `ENV_FLUENTBIT_EXPORTER_IMAGE` tag | `{DIR_SIZE_IMAGE_TAG}`     |

- Generated files (such as Helm chart manifests) updated by `make generate`.

After you merge the PR, the workflow resumes.

### 6. Automatic Testing

After you merge the PR, the release workflow automatically runs the following tests:

1. **Unit Tests**: Full test suite
2. **PR Integration Tests**: End-to-end integration tests
3. **Gardener Integration Tests**: Tests on Gardener-managed clusters
4. **Release Report Upload**: Compliance report upload

All tests must pass before the release workflow proceeds to release creation.

### 7. Automatic Tag and Release Creation

After all tests pass, the release workflow creates the release by performing the following actions:

1. Creates annotated Git tag: **`{VERSION}`**
2. Pushes the tag to trigger the following processes:
   - The release workflow uses `build-manager-image.yml` to build and push the Docker image
   - It uses goreleaser to create the release
3. Packages Helm chart
4. Uploads Helm chart to the GitHub release
5. Updates `gh-pages` branch with Helm repository index

### 8. Automatic Module Releases

If `module_release` is set to `true` (the default), the workflow triggers module releases after it creates the GitHub release.

The workflow triggers module releases for the following channels:

| Channel        | Auto-merge | Target Repository       |
|----------------|------------|-------------------------|
| `fast`         | Enabled    | `kyma/module-manifests` |
| `experimental` | Enabled    | `kyma/module-manifests` |

### 9. Verify the Release

After the release completes, perform the following tasks:
- To verify the release, check [Releases](https://github.com/kyma-project/telemetry-manager/releases). A successful release produces the following artifacts:
    - Git tag: **`{VERSION}`**
    - GitHub release with auto-generated changelog
    - Docker image: `europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:`**`{VERSION}`**
    - Helm chart: `telemetry-`**`{VERSION}`**`.tgz`, attached to GitHub release
    - Module manifest PRs in `kyma/module-manifests` repository (only if `module_release=true`)
  - Review the auto-generated release notes. If you cherry-picked commits for the release, some changes might appear duplicated. Edit the release notes to correct this.

## Release to the Regular Channel

To release to the regular channel, manually trigger the module release workflow:

In the telemetry-manager repository, go to **Actions**, select [Telemetry Module Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml), and run the workflow with the following inputs:
   - **version**: **`{VERSION}`**, such as `1.2.3`
   - **channel**: `regular`
   - **dry_run**: `false`
   - **auto_merge**: `true` or `false` for manual merge

## Monitor Release Progress

### Workflow Status

Monitor the release workflow at: [Actions > Telemetry Manager Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml)

**Key Jobs**:
1. `validate-and-prepare-branch`: Validation and branch setup
2. `prepare-release`: Version bump PR creation and merge wait
3. `unit-tests`: Test execution
4. `run-pr-integration`: Integration tests
5. `run-gardener-integration`: Gardener tests
6. `upload-release-report`: Compliance reporting
7. `create-release`: Final release creation

### Module Release Status

Monitor module releases at: [Actions > Telemetry Module Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml)

Check the pull requests for both experimental and fast channels in the [module-manifests repository](https://github.tools.sap/kyma/module-manifests/pulls).

## Troubleshooting

### Milestone Validation Error

**Symptom:** The release workflow fails with a milestone validation error.

**Solution:**
1. Go to [Milestones](https://github.com/kyma-project/telemetry-manager/milestones) and close any open issues.
2. With all issues closed, close the milestone for the release version.
3. Rerun the release workflow.

### Docker Image Not Found

**Symptom:** The release workflow fails because a Docker image was not found.

**Solution:**
1. Check that the image exists in the registry.
2. Check that the image tag format matches the expected pattern.
3. If the image is missing, build the missing image by running the corresponding workflow, and wait for the workflow to complete.
4. Rerun the release workflow.

### GitHub Tag Already Exists

**Symptom:** The release workflow fails because the GitHub tag already exists.

**Solution:**
Rerun the release workflow with force mode to recreate the release.

> [!CAUTION]
> This overwrites the existing release if it exists.

### Version Bump PR Times Out

**Symptom:** The release workflow fails because the PR was not merged within 120 minutes.

**Solution:**
1. Review and merge the PR manually.
2. In the telemetry-manager repository, go to **Actions** and rerun the release workflow.

## Related Workflows

- [Module Release Workflow](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml)
- [Management Plane Chart Release Workflow](https://github.com/kyma-project/telemetry-manager/actions/workflows/mpc-release.yml) 
