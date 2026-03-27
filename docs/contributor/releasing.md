# Release Process

This document describes the automated release process for Telemetry Manager using GitHub Actions workflows.

## Overview

The release process uses GitHub Actions workflows to automate the following tasks:
- Version validation and milestone verification
- Release branch creation and version bumping
- Docker image builds and tests
- GitHub release creation
- Module manifest updates for multiple channels

## Prerequisites


1. **Milestone Verification**: All issues in the [GitHub milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the version are closed and the milestone is closed.

2. **Component Releases**: The following component dependencies are already released in the specified order:
   1. [directory-size-exporter](https://github.com/kyma-project/telemetry-manager/actions/workflows/build-directory-size-reporter-image.yml) - produces image tags like `v20260302-12345678`
   2. [telemetry-self-monitor](https://github.com/kyma-project/telemetry-manager/actions/workflows/build-self-monitor-image.yml) - produces image tags like `v20260302-bbf32a3b`
   3. [opentelemetry-collector-components](https://github.com/kyma-project/opentelemetry-collector-components) - version format: `{OCC_VERSION}-{TELEMETRY_VERSION}` (for example, `0.100.0-1.2.3`)

3. **Docker Image Availability**: Verify that the required Docker images exist:
   ```bash
   # Check OCC image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:{OCC_VERSION}

   # Check directory-size-exporter image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:{DIR_SIZE_TAG}

   # Check self-monitor image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/tpi/telemetry-self-monitor:{SELF_MONITOR_TAG}
   ```

4. **Access Requirements**: You have the following permissions:
   - Write access to the telemetry-manager repository
   - Access to merge PRs on the release branch

## Release Steps

### Step 1: Start Release Workflow

In the telemetry-manager repo, go to **Actions**, select [Telemetry Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml) and run the workflow with the following inputs:


| Input                      | Description                                                             | Example              |
|----------------------------|-------------------------------------------------------------------------|----------------------|
| **version**                | Release version in X.Y.Z format                                         | `1.2.3`              |
| **occ_image_version**      | OCC image version in X.Y.Z-A.B.C format                                 | `0.100.0-1.2.3`      |
| **self_monitor_image_tag** | Self-monitor image tag in vYYYYMMDD-HASH format                         | `v20260302-bbf32a3b` |
| **dir_size_image_tag**     | Directory size exporter image tag in vYYYYMMDD-HASH format              | `v20260302-12345678` |
| **dry_run**                | Test the release process without creating tags/releases                 |                      |
| **force**                  | Recreate existing release (use with caution)                            |                      |
| **module_release**         | Trigger module release for experimental and fast channels after release |                      |

To test the release process without creating actual tags or releases, set `dry_run` to `true`. This allows you to validate the workflow and catch any issues before performing the real release.
The `force` option allows you to re-create an existing release by deleting the existing tag and release before creating a new one. Use this option with caution, as it overwrites the existing release.
The `module_release` option controls whether to automatically trigger module releases for the experimental and fast channels after the main release is created. The workflow by default will create module release for experimental and fast channel, set it to `false` if you want to skip module releases or trigger them manually later.

> [!CAUTION]
> Force mode deletes the existing release and tag before recreating them. Use it only when necessary and communicate with the team beforehand.

Consider using force mode for the following purposes:
- Fixing a broken release
- Updating release assets
- Correcting version metadata

### Step 2: Workflow Validation Phase

The workflow automatically validates the following conditions:

- The version format follows semantic versioning (`X.Y.Z`).
- The OCC version format matches the expected pattern (`X.Y.Z-A.B.C`).
- The image tag format matches the expected pattern (`vYYYYMMDD-HASH`).
- All required Docker images exist in the registry.
- The milestone exists, is closed, and has no open issues.
- No existing release or tag conflicts with the target version (skipped if force mode is enabled).
- The release branch exists for a patch release, or a new branch is needed for a minor/major release.

If validation fails, the workflow stops and reports the error.

### Step 3: Release Branch Preparation

To determine the release type, the workflow checks if a `release-X.Y` branch already exists and handles branch preparation:

- For a minor or major release, the `release-X.Y` branch does not exist, so the workflow creates it from the `main` branch.

-  For a patch release, the `release-X.Y` branch already exists, so the workflow uses the existing branch.

### Step 4: Version Bump PR

The workflow creates a pull request (PR) against the release branch. This PR updates all version numbers and image tags for the new release.

First, the workflow updates the following variables in the `.env` file with the values you provided:
  - `ENV_HELM_RELEASE_VERSION={VERSION}`
  - `ENV_MANAGER_IMAGE` tag to `{VERSION}`
  - `ENV_OTEL_COLLECTOR_IMAGE` tag to `{OCC_IMAGE_VERSION}`
  - `ENV_SELFMONITOR_IMAGE` tag to `{SELF_MONITOR_IMAGE_TAG}`
  - `ENV_FLUENTBIT_EXPORTER_IMAGE` tag to `{DIR_SIZE_IMAGE_TAG}`
Next, it runs the `make generate` command to apply these changes to all auto-generated files, such as the Helm chart manifests.

> [!WARNING] 
> The workflow waits up to 120 minutes for you to review and merge the PR. If you do not merge the PR within that time, the workflow times out and fails.
To review the PR, use the checklist in its description to verify the following conditions:
- [ ] Version numbers are correct
- [ ] Generated files are up to date
- [ ] No unintended changes

### Step 5: Automated Testing

After you merged the PR, the workflow automatically runs the following tests:

1. **Unit Tests**: Full test suite
2. **PR Integration Tests**: End-to-end integration tests
3. **Gardener Integration Tests**: Tests on Gardener-managed clusters
4. **Release Report Upload**: Uploads compliance reports

All tests must pass before proceeding to release creation.

### Step 6: Release Tag and GitHub Release

After all tests pass, the workflow performs the following actions to create the release:

1. Creates annotated Git tag: `{VERSION}`
2. Pushes tag to trigger the following processes:
   - `build-manager-image.yml`: Builds and pushes Docker image
   - Release creation via goreleaser
3. Packages Helm chart
4. Uploads Helm chart to the GitHub release
5. Updates `gh-pages` branch with Helm repository index

### Step 7: Module Releases (Conditional)

If `module_release` is set to `true` (default), the workflow triggers module releases after it created the GitHub release.

**Fast Channel**:
- Triggers `module-release.yml` workflow
- Channel: `fast`
- Auto-merge: enabled
- Creates PR in `kyma/module-manifests` repository

**Experimental Channel**:
- Triggers `module-release.yml` workflow
- Channel: `experimental`
- Auto-merge: enabled
- Creates PR in `kyma/module-manifests` repository

If all checks pass, both PRs merge automatically.

> [!NOTE] 
> To manually trigger module releases later or skip them entirely, set `module_release` to `false`.

### Step 8: Regular Channel (Manual)

To release to the regular channel, manually trigger the module release workflow:

In the telemetry-manager repo, go to **Actions** and select [Telemetry Module Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml) and run the workflow with the following inputs:
   - **version**: `{VERSION}` (for example, `1.2.3`)
   - **channel**: `regular`
   - **dry_run**: `false`
   - **auto_merge**: `true` (or `false` for manual merge)

## Release Channels

| Channel          | Purpose              | Update Frequency  | Trigger                            |
|------------------|----------------------|-------------------|------------------------------------|
| **experimental** | Testing new features | Every release     | Automatic (if module_release=true) |
| **fast**         | Early adopters       | Every release     | Automatic (if module_release=true) |
| **regular**      | Stable production    | Selected releases | Manual                             |

## Monitoring Release Progress

### Workflow Status

Monitor the release workflow at: [Actions > Telemetry Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml)

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

Check PRs in the module-manifests repository, for both experimental and fast channels [here](https://github.tools.sap/kyma/module-manifests/pulls).

## Troubleshooting

### Workflow Validation Failures

**Symptom:** The workflow fails with a milestone validation error.

**Solution:**
1. Go to [Milestones](https://github.com/kyma-project/telemetry-manager/milestones) and close any open issues.
2. With all issues closed, close the milestone for the release version.
3. Rerun the workflow.

**Symptom**: The workflow fails with an error that a Docker image was not found.

**Solution:**
1. Check that the image exists in registry.
2. Check that the image tag format matches the expected pattern.
3. If the image is missing, build missing image(s) by running the corresponding workflow, wait for it to complete.
4. Rerun the release workflow.

**Symptom**: The workflow fails with an error that the tag already exists.

**Solution:**
1. To check the existing tags, run: `git tag -l {VERSION}`
2. Remove the existing tag manually if it is a leftover from a failed release attempt.
3. Rerun the release workflow.

Alternative, rerun with force mode to re-create (caution: overwrites existing release if exists)

### Version Bump PR Times Out

**Cause**:
The workflow waits for a maximum of 120 minutes for the PR to be merged. If you don't merge it in time, the workflow fails.
**Solution**
1. Review and merge the PR manually.
2. In the telemetry-manager repo, go to **Actions** and rerun the workflow.

## Post-Release Tasks

After the release is complete, perform the following tasks:
  - To verify the release, check [Releases](https://github.com/kyma-project/telemetry-manager/releases). A successful release produces the following artifacts:
    - Git tag: `{VERSION}`
    - GitHub release with auto-generated changelog
    - Docker image: `europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:{VERSION}`
    - Helm chart: `telemetry-{VERSION}.tgz` (attached to GitHub release)
    - Module manifest PRs in `kyma/module-manifests` repository for experimental and fast channels (if module_release=true)
  - Create milestone for next version at [Milestones](https://github.com/kyma-project/telemetry-manager/milestones). 
  - Review the auto-generated release notes. If you cherry-picked commits for the release, some changes might appear duplicated. Edit the release notes to correct this.

## Related Documentation

- [Release Workflow](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml) - Main release workflow
- [Module Release Workflow](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml) - Module release automation

