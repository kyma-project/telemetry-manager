# Release Process

This document describes the automated release process for Telemetry Manager using GitHub Actions workflows.

## Overview

The release process is automated through GitHub Actions workflows that handle:
- Version validation and milestone verification
- Release branch creation and version bumping
- Docker image builds and tests
- GitHub release creation
- Module manifest updates for multiple channels

## Prerequisites

Before starting a release, ensure:

1. **Milestone Verification**: All issues in the [GitHub milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the version are closed and the milestone is closed. Create a new [GitHub milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the next version.

2. **Component Releases**: Release dependencies in this order:
   - [directory-size-exporter](https://github.com/kyma-project/telemetry-manager/actions/workflows/build-directory-size-reporter-image.yml) - produces image tags like `v20260302-12345678`
   - [telemetry-self-monitor](https://github.com/kyma-project/telemetry-manager/actions/workflows/build-self-monitor-image.yml) - produces image tags like `v20260302-bbf32a3b`
   - [opentelemetry-collector-components](https://github.com/kyma-project/opentelemetry-collector-components) - version format: `{OCC_VERSION}-{TELEMETRY_VERSION}` (for example, `0.100.0-1.2.3`)

3. **Docker Image Availability**: Verify that the required Docker images exist:
   ```bash
   # Check OCC image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:{OCC_VERSION}

   # Check directory-size-exporter image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/directory-size-exporter:{DIR_SIZE_TAG}

   # Check self-monitor image
   docker manifest inspect europe-docker.pkg.dev/kyma-project/prod/tpi/telemetry-self-monitor:{SELF_MONITOR_TAG}
   ```

4. **Access Requirements**:
   - Write access to the telemetry-manager repository
   - Access to merge PRs on the release branch

## Release Steps

### Step 1: Start Release Workflow

Navigate to [Actions > Telemetry Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml) and click **Run workflow**.

Provide the following inputs:

| Input | Description | Example |
|-------|-------------|---------|
| **version** | Release version in X.Y.Z format | `1.2.3` |
| **occ_image_version** | OCC image version in X.Y.Z-A.B.C format | `0.100.0-1.2.3` |
| **self_monitor_image_tag** | Self-monitor image tag in vYYYYMMDD-HASH format | `v20260302-bbf32a3b` |
| **dir_size_image_tag** | Directory size exporter image tag in vYYYYMMDD-HASH format | `v20260302-12345678` |
| **dry_run** | Test the release process without creating tags/releases | `false` |
| **force** | Re-create existing release (use with caution) | `false` |
| **module_release** | Trigger module release for experimental and fast channels after release | `true` |

### Step 2: Workflow Validation Phase

The workflow automatically performs these validations:

1. **Version Format Check**: Validates version follows semantic versioning (X.Y.Z)
2. **OCC Version Check**: Validates OCC version format (X.Y.Z-A.B.C)
3. **Image Tag Check**: Validates image tag formats (vYYYYMMDD-HASH)
4. **Docker Image Verification**: Checks that all required images exist in the registry
5. **Milestone Verification**: Ensures milestone exists, is closed, and has no open issues
6. **Release/Tag Existence Check**: Prevents duplicate releases (unless force mode is enabled)
7. **Release Branch Determination**: Identifies if this is a patch release (branch exists) or minor/major release (new branch needed)

If validation fails, the workflow stops and reports the error.

### Step 3: Release Branch Preparation

The workflow automatically:

**For Minor/Major Releases** (release branch doesn't exist):
- Creates release branch: `release-X.Y` (for example, `release-1.2`)
- Branches from `main`

**For Patch Releases** (release branch exists):
- Uses existing release branch
- No new branch creation needed

### Step 4: Version Bump PR

The workflow creates a PR to the release branch with:

**Changes**:
- Updates `.env` file:
  - `ENV_HELM_RELEASE_VERSION={VERSION}`
  - `ENV_MANAGER_IMAGE` tag to `{VERSION}`
  - `ENV_OTEL_COLLECTOR_IMAGE` tag to `{OCC_IMAGE_VERSION}`
  - `ENV_SELFMONITOR_IMAGE` tag to `{SELF_MONITOR_IMAGE_TAG}`
  - `ENV_FLUENTBIT_EXPORTER_IMAGE` tag to `{DIR_SIZE_IMAGE_TAG}`
- Runs `make generate` to update generated files

**Action Required**:
The workflow waits (up to 120 minutes) for you to review and merge the PR. The PR includes a checklist:
- [ ] Version numbers are correct
- [ ] Generated files are up to date
- [ ] No unintended changes

### Step 5: Automated Testing

After the PR is merged, the workflow automatically runs:

1. **Unit Tests**: Full test suite execution
2. **PR Integration Tests**: End-to-end integration tests
3. **Gardener Integration Tests**: Tests on Gardener-managed clusters
4. **Release Report Upload**: Uploads compliance reports

All tests must pass before proceeding to release creation.

### Step 6: Release Tag and GitHub Release

After successful tests, the workflow:

1. Creates annotated Git tag: `{VERSION}`
2. Pushes tag to trigger:
   - `build-manager-image.yml`: Builds and pushes Docker image
   - Release creation via goreleaser
3. Packages Helm chart
4. Uploads Helm chart to the GitHub release
5. Updates `gh-pages` branch with Helm repository index

### Step 7: Module Releases (Conditional)

If `module_release` is set to `true` (default), the workflow automatically triggers module releases after the GitHub release is created:

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

Both PRs will auto-merge when all checks pass.

**Note**: Set `module_release: false` if you want to manually trigger module releases later or skip them entirely.

### Step 8: Regular Channel (Manual)

For the regular channel, manually trigger the module release:

1. Navigate to [Actions > Telemetry Module Release](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml)
2. Click **Run workflow**
3. Provide:
   - **version**: `{VERSION}` (for example, `1.2.3`)
   - **channel**: `regular`
   - **dry_run**: `false`
   - **auto_merge**: `true` (or `false` for manual merge)

## Release Channels

| Channel | Purpose | Update Frequency | Trigger |
|---------|---------|------------------|---------|
| **experimental** | Testing new features | Every release | Automatic (if module_release=true) |
| **fast** | Early adopters | Every release | Automatic (if module_release=true) |
| **regular** | Stable production | Selected releases | Manual |

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

Check PRs in the module-manifests repository:
- For fast channel: `https://github.tools.sap/kyma/module-manifests/pulls`
- For experimental channel: `https://github.tools.sap/kyma/module-manifests/pulls`

## Troubleshooting

### Workflow Validation Failures

**Milestone not closed**:
- Close the milestone at [Milestones](https://github.com/kyma-project/telemetry-manager/milestones)
- Ensure all issues are closed first

**Docker image not found**:
- Verify image exists in registry
- Check image tag format matches expected pattern
- Wait for dependency builds to complete

**Tag already exists**:
- Check existing tags: `git tag -l {VERSION}`
- Use force mode to re-create (caution: overwrites existing release)
- Or choose a different version number

### PR Not Merging

If the version bump PR is not merged within 120 minutes:
- Workflow times out and fails
- Review and merge the PR manually
- Re-run the workflow from the GitHub Actions UI

### Module Release Issues

**PR creation fails**:
- Check `HUSKIES_GHTOOLS_TOKEN` secret is configured
- Verify network access to github.tools.sap
- Check workflow logs for detailed error messages

**Auto-merge not working**:
- Verify branch protection rules allow auto-merge
- Check that required checks are passing
- Manually merge if auto-merge fails

## Dry Run Mode

To test the release process without creating actual releases:

1. Set `dry_run: true` when starting the workflow
2. The workflow will:
   - Perform all validations
   - Create and test version bump changes locally
   - Run all tests
   - Skip tag creation, release creation, and PR creation
3. Review the dry run summary in the workflow output
4. Run again with `dry_run: false` to create the actual release

## Force Mode

Use force mode to re-create an existing release (use with caution):

1. Set `force: true` when starting the workflow
2. The workflow will:
   - Delete existing release and tag
   - Proceed with release creation
3. Use cases:
   - Fixing a broken release
   - Updating release assets
   - Correcting version metadata

**Warning**: Force mode overwrites existing releases. Use only when necessary and communicate with the team.

## Post-Release Tasks

After a successful release:

1. **Verify Release**: Check [Releases](https://github.com/kyma-project/telemetry-manager/releases) page
2. **Monitor Module PRs**: Ensure module release PRs merge successfully
3. **Announce Release**: Notify the team via appropriate channels
4. **Update Documentation**: Update external documentation if API/features changed
5. **Create Next Milestone**: Create milestone for next version at [Milestones](https://github.com/kyma-project/telemetry-manager/milestones)
6. **Update Release Notes**: Edit release notes if cherry-picked changes appear duplicated

## Release Artifacts

A successful release produces:

- **Git Tag**: `{VERSION}` (for example, `1.2.3`)
- **Docker Image**: `europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:{VERSION}`
- **Helm Chart**: `telemetry-{VERSION}.tgz` (attached to GitHub release)
- **GitHub Release**: With auto-generated changelog
- **Module Manifests**: PRs in module-manifests repository for each channel

## Changelog

The release changelog is auto-generated from PR titles following [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

### PR Title Format

`type(scope?): subject`

**Types**:
- `feat`: New feature or functionality change
- `fix`: Bug or regression fix
- `docs`: Documentation changes
- `test`: Test suite changes
- `deps`: External dependency changes
- `chore`: Maintenance changes (not included in changelog)

**Subject Guidelines**:
- Use imperative mood (Add, Fix, Update, not Added, Fixed, Updated)
- Start with uppercase
- No period at the end
- Follow Kyma [capitalization](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/04-style-and-terminology.md#capitalization) and [terminology](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/04-style-and-terminology.md#terminology) guides

## Related Documentation

- [Release Workflow](https://github.com/kyma-project/telemetry-manager/actions/workflows/release.yml) - Main release workflow
- [Module Release Workflow](https://github.com/kyma-project/telemetry-manager/actions/workflows/module-release.yml) - Module release automation

