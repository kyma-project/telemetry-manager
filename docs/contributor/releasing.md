# Release Process

1. Create a release branch in the `telemetry-manager` repository. The name of this branch must follow the `release-x.y` pattern, such as `release-1.0`.

   >**NOTE:** This step applies only to new major and minor versions.

   ```bash
   git fetch upstream
   git checkout --no-track -b {RELEASE_NAME} upstream/main
   git push upstream {RELEASE_NAME}
   ```

2. Create a PR to `telemetry-manager/release-x.y` with the following changes:
    - `Makefile`:
        - Ensure that the `IMG` variable is up to date with the latest `telemetry-manager` version.
        - Update the `MODULE_VERSION` variable to the value of the new module version.
    - `config/manager/kustomization.yaml`:
        - Ensure that the `newTag` field for the `telemetry-manager` image is up to date with the latest version.
    - `sec-scanners-config.yaml`:
        - Ensure that all of the images listed in the `protecode` field have the same versions as the images used in the `main.go` file.
3. After merging the PR, create a tag on the release branch that has the value of the new module release version.

    ```bash
    git tag -a {RELEASE_VERSION} -m "Release {RELEASE_VERSION}"
    ```

    Replace {RELEASE_VERSION} with the new module release version, for example, `1.0.0`.

4. Push the tag to trigger a postsubmit job (`post-telemetry-manager-release-module`) that creates the GitHub release.

    ```bash
    git push upstream {RELEASE_VERSION}
    ```

5. Verify the [Prow Status](https://status.build.kyma-project.io/) of the postsubmit job (`post-telemetry-manager-release-module`). Once the postsubmit job succeeds, the new Github release should be available under [releases](https://github.com/kyma-project/telemetry-manager/releases). If the postsubmit job failed, you can re-trigger it by removing the tag from upstream and pushing it again.

    ```bash
    git push --delete upstream {RELEASE_VERSION}
    git push upstream {RELEASE_VERSION}
    ``` 