# Release Process

This release process covers the steps to release new major and minor versions for the Kyma Telemetry module.

1. Verify that all issues in the [Github milestone](https://github.com/kyma-project/telemetry-manager/milestones) related to the version are closed and close the milestone.

1. Create a new [Github milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the next version.

1. Bump the `telemetry-manager/main` branch with the new versions for the dependent images.
   Create a PR to `telemetry-manager/main` with the following changes:
   - `Makefile`:
      - Ensure the `IMG` variable reflects the latest `telemetry-manager` version.
      - Update the `MODULE_VERSION` variable to the value of the new module version.
   - `config/manager/kustomization.yaml`:
      - Ensure the `newTag` field for the `telemetry-manager` image reflects the latest version.
   - `sec-scanners-config.yaml`:
      - Ensure that all images listed in the `protecode` field have the same versions as those used in the `main.go` file.

1. Merge the PR.

1. In the `telemetry-manager` repository, create a release branch.
   The name of this branch must follow the `release-x.y` pattern, such as `release-1.0`.
   ```bash
   git fetch upstream
   git checkout --no-track -b {RELEASE_BRANCH} upstream/main
   git push upstream {RELEASE_BRANCH}
   ```

1. In the `telemetry-manager/{RELEASE_BRANCH}` branch, create a release tag for the head commit.
   ```bash
   git tag -a {RELEASE_VERSION} -m "Release {RELEASE_VERSION}"
   ```
   Replace {RELEASE_VERSION} with the new module release version, for example, `1.0.0`. The release tag points to the HEAD commit in `telemetry-manager/main` and `telemetry-manager/{RELEASE_BRANCH}` branches.

1. Push the tag to trigger a postsubmit job (`post-telemetry-manager-release-module`) that creates the GitHub release.
   ```bash
   git push upstream {RELEASE_VERSION}
   ```

1. Verify the [Prow Status](https://status.build.kyma-project.io/) of the postsubmit job (`post-telemetry-manager-release-module`).
   - Once the postsubmit job succeeds, the new Github release is available under [releases](https://github.com/kyma-project/telemetry-manager/releases).
   - If the postsubmit job failed, retrigger it by removing the tag from upstream and pushing it again:
     ```bash
     git push --delete upstream {RELEASE_VERSION}
     git push upstream {RELEASE_VERSION}
     ```
