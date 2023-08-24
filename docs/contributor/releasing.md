# Releasing

## Release Process

This release process covers the steps to release new major and minor versions for the Kyma Telemetry module.

1. Verify that all issues in the [Github milestone](https://github.com/kyma-project/telemetry-manager/milestones) related to the version are closed.

2. Close the milestone.

3. Create a new [Github milestone](https://github.com/kyma-project/telemetry-manager/milestones) for the next version.

4. Bump the `telemetry-manager/main` branch with the new versions for the dependent images.
   Create a PR to `telemetry-manager/main` with the following changes:
   - `Makefile`:
      - Ensure the `IMG` variable reflects the latest `telemetry-manager` version.
      - Update the `MODULE_VERSION` variable to the value of the new module version.
   - `config/manager/kustomization.yaml`:
      - Ensure the `newTag` field for the `telemetry-manager` image reflects the latest version.
   - `sec-scanners-config.yaml`:
      - Ensure that all images listed in the `protecode` field have the same versions as those used in the `main.go` file.

5. Merge the PR.

6. In the `telemetry-manager` repository, create a release branch.
   The name of this branch must follow the `release-x.y` pattern, such as `release-1.0`.
   ```bash
   git fetch upstream
   git checkout --no-track -b {RELEASE_BRANCH} upstream/main
   git push upstream {RELEASE_BRANCH}
   ```

7. In the `telemetry-manager/{RELEASE_BRANCH}` branch, create a release tag for the head commit.
   ```bash
   git tag -a {RELEASE_VERSION} -m "Release {RELEASE_VERSION}"
   ```
   Replace {RELEASE_VERSION} with the new module release version, for example, `1.0.0`. The release tag points to the HEAD commit in `telemetry-manager/main` and `telemetry-manager/{RELEASE_BRANCH}` branches.

8. Push the tag to trigger a postsubmit job (`post-telemetry-manager-release-module`) that creates the GitHub release.
   ```bash
   git push upstream {RELEASE_VERSION}
   ```

9. Verify the [Prow Status](https://status.build.kyma-project.io/) of the postsubmit job (`post-telemetry-manager-release-module`).
   - Once the postsubmit job succeeds, the new Github release is available under [releases](https://github.com/kyma-project/telemetry-manager/releases).
   - If the postsubmit job failed, retrigger it by removing the tag from upstream and pushing it again:
     ```bash
     git push --delete upstream {RELEASE_VERSION}
     git push upstream {RELEASE_VERSION}
     ```

## Changelog

Every PR's title must adhere to the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification for an automatic changelog generation. It is enforced by a [Conventional Pull Request](https://github.com/marketplace/actions/conventional-pull-request) GitHub Action and a [Commitlint](https://commitlint.js.org/#/reference-rules).

### Pull Request Title

Due to the Squash merge Github Workflow, each PR results in a single commit after merging into the main development branch. The PR's title becomes the commit message and must adhere to the template:

`type(scope?): subject`

#### Type

* **feat**. A new feature or functionality change.
* **fix**. A bug or regression fix.
* **docs**. Changes regarding the documentation.
* **test**. The test suite alternations.
* **deps**. The changes in the external dependencies.
* **chore**. Anything not covered by the above categories (e.g. refactoring or artefacts building alternations).

Beware that PRs of type `chore` do not appear in the Changelog for the release. Therefore, exclude maintenance changes that are not interesting to consumers of the project by marking them with chore type:

* Dotfile changes (.gitignore, .github and so forth).
* Changes to development-only dependencies.
* Minor code style changes.
* Formatting changes in documentation.

#### Subject

The subject must describe the change and follow the recommendations:

* Describe a change using the [imperative mood](https://en.wikipedia.org/wiki/Imperative_mood).  It must start with a present-tense verb, for example (but not limited to) Add, Document, Fix, Deprecate.
* Start with an uppercase.
* Kyma [capitalization](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/02-style-and-terminology.md#capitalization) and [termonology](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/02-style-and-terminology.md#terminology) guides.
