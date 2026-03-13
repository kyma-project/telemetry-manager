# Governance

Some quality aspects are covered by automated verification, so you must locally execute tooling before a commitment. These aspects are outlined in this document.

## CRD Generation

The Telemetry Manager uses Kubernetes CRDs that are defined as Golang source code in the [apis](../../apis) folder. Before you install the CRDs with Helm for the Telemetry Manager deployment, generate the Kubernetes manifest files into the [templates](../../helm/charts/default/templates) folder. Additionally, update the user documentation in the [resources](../user/resources/) folder.

To achieve both aspects, call:

```shell
make manifests
```

Additionally, a [Github Action](../../.github/workflows/pr-code-checks.yml) verifies this operation.

## Sourcecode Linting

For the source code linting, the development team uses [golangci-lint](https://golangci-lint.run) with fine-grained configuration.

Additionally, a [Github Action](../../.github/workflows/pr-code-checks.yml) verifies this operation.

### Linters in Action

The following linters are configured and integrated as a CI stage using a [Github Action](../../.github/workflows/pr-code-checks.yml).

<details>
<summary>List of Linters</summary>
<br>

| Linter         | Description                                         | [Suppress](#nolint) |
| -------------- | --------------------------------------------------- | ------------------- |
| asasalint      | check for pass []any as any in variadic func        | inline //nolint     |
| asciicheck     | checks for non-ASCII identifiers                    | inline //nolint     |
| bodyclose      | checks whether HTTP response body is closed         | inline //nolint     |
| dogsled        | checks assignments with too many blank identifiers  | inline //nolint     |
| dupl           | checks for code clone detection                     |                     |
| dupword        | checks for duplicate words in the source code       | inline //nolint     |
| errcheck       | checks for unhandled errors                         | inline //nolint     |
| errchkjson     | checks types passed to the json encoding functions  | inline //nolint     |
| exportloopref  | finds exporting pointers for loop variables         | inline //nolint     |
| gci            | checks import order and ensures determinism         | inline //nolint     |
| ginkgolinter   | enforces standards of using Ginkgo and Gomega       | inline //nolint     |
| gocheckcompilerdirectives | checks go compiler directive comments    | inline //nolint     |
| gochecknoinits | checks that no init functions are present           | inline //nolint     |
| gofmt          | checks whether code was gofmt'ed                    |                     |
| goimports      | check import statements formatting                  | inline //nolint     |
| gosec          | inspects source code for security problems          | inline //nolint     |
| govet          | examines Go source code for suspicious constructs   | inline //nolint     |
| ineffassign    | detects when assignments to variables are not used  | inline //nolint     |
| loggercheck    | checks key-value pairs for logger libraries         | inline //nolint     |
| misspell       | finds commonly misspelled English words in comments | inline //nolint     |
| nolintlint     | reports ill-formed or insufficient nolint directives| inline //nolint     |
| revive         | comprehensive golint replacement                    | inline //nolint     |
| staticcheck    | performs static code analysis                       | inline //nolint     |
| stylecheck     | examines Go code-style conformance                  | inline //nolint     |
| typecheck      | parses and type-checks Go code                      | inline //nolint     |
| unparam        | reports unused function parameters                  | inline //nolint     |
| unused         | checks for unused constants, variables, functions   | inline //nolint     |

</details>

### Irrelevant Linters

The following linters are irrelevant for development of the Telemetry module:

<details>
<summary>List of irrelevant linters</summary>
<br>

| Linter             | Reason                               |
| ------------------ | ------------------------------------ |
| `bidichk`          | superseded by `stylecheck`           |
| `deadcode`         | superseded by `unused`               |
| `execinquery`      | `database/sql` package is not used   |
| `exhaustivestruct` | superseded by `exhaustruct`          |
| `forcetypeassert`  | superseded by `errcheck`             |
| `golint`           | superseded by `revive`, `stylecheck` |
| `ifshort`          | deprecated                           |
| `interfacer`       | deprecated                           |
| `maligned`         | superseded by `govet`                |
| `nosnakecase`      | superseded by `revive`               |
| `rowserrcheck`     | `database/sql` package is not used   |
| `sqlclosecheck`    | `database/sql` package is not used   |
| `scopelint`        | superseded by `exportloopref`        |
| `structcheck`      | superseded by `unused`               |
| `testableexamples` | Go Example functions are not used    |
| `varcheck`         | superseded by `unused`               |
| `wastedassign`     | superseded by `inefassign`           |

</details>

### Nolint

If linting warnings add noise to the development or are routinely ignored (for example, because some linters produce false-positive warnings), consider disabling the linter.
You can either suppress one or more lines of code inline, or exclude whole files.

> **TIP:** The benefit of declaring linter exclusion rules on a file basis in the config file and not as package-level inline nolint instructions is that it is easier to visualize and analyse the linting suppressions.

#### Suppress Linting Warnings Inline With //nolint

To suppress a linting warning for a particular line of code, use nolint instruction `//no-lint:{LINTER} // {COMMENT}.` For the _LINTER_ and _COMMENT_ placeholders, you must enter the linter to be suppressed and a reason for the suppression.

To suppress linting warnings for the whole block, use preceding inline nolint comments. The following example suppresses the linter for the entire file:

   ```go
   //nolint:errcheck // The error check should be suppressied for the module.
   package config

   // The rest of the file will not be linted by errcheck.
   ```

#### Suppress Linting Warnings for Whole Files

To prevent some files from being linted, use the section `.issues.exclude-rules` in the `.golangci.yaml` configuration file.

The code duplication linting scenario is problematic for being disabled an a per line basis. The following example suppresses the `dupl` linter for the set of three files in the _apis/telemetry/v1alpha1_ module:

   ```yaml
   issues:
     exclude-rules:
       - linters: [ dupl ]
         path: apis/telemetry/v1alpha1/(metricpipeline|tracepipeline)_types_test.go
   ```

### Dev Environment Configuration

Read [golangci-lint integrations](https://golangci-lint.run/docs/welcome/integrations/) for information on configuring file-watchers for your IDE or code editor.

### Autofix

Some of the linting errors can be automatically fixed with the command:
`make lint_autofix`
