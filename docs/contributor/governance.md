# Governance

Some quality aspects are covered by automated verification, so you must locally execute tooling before a commitment. These aspects are outlined in this document.

## CRD Generation

The API of Telemetry Manager is realized by Kubernetes CRDs defined in the [apis](../../apis) folder as Golang source code. To install the CRDs later using Kustomize together with Telemetry Manager deployment, you must generate proper Kubernetes [manifest files](../../config/crd/bases). Also, you must update the [user documentation](../user/resources/).

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

| Linter | Description | [Suppress](#nolint) |
| --- | --- | --- |
| [`asasalint`](https://github.com/alingse/asasalint) [⛭](https://golangci-lint.run/usage/linters/#asasalint)                                                       | check for pass []any as any in variadic func                     | inline //nolint |
| [`asciicheck`](https://github.com/tdakkota/asciicheck)                                                                                                            | checks for non-ASCII identifiers                                 | inline //nolint |
| [`bodyclose`](https://github.com/timakin/bodyclose)                                                                                                               | checks whether HTTP response body is closed successfully         | inline //nolint |
| [`dogsled`](https://github.com/alexkohler/dogsled) [⛭](https://golangci-lint.run/usage/linters/#dogsled)                                                          | checks assignments with too many blank identifiers               | inline //nolint |
| [`dupl`](https://github.com/mibk/dupl) [⛭](https://golangci-lint.run/usage/linters/#dupl)                                                                         | checks for code clone detection                                  |                                    |
| [`dupword`](https://github.com/Abirdcfly/dupword) [⛭](https://golangci-lint.run/usage/linters/#dupword)                                                           | checks for duplicate words in the source code                    | inline //nolint |
| [`errcheck`](https://github.com/kisielk/errcheck) [⛭](https://golangci-lint.run/usage/linters/#errcheck)                                                          | checks for unhandled errors                                      | inline //nolint |
| [`errchkjson`](https://github.com/breml/errchkjson) [⛭](https://golangci-lint.run/usage/linters/#errchkjson)                                                      | checks types passed to the json encoding functions               | inline //nolint |
| `exportloopref`                                                                                                                                                   | finds exporting pointers for loop variables                      | inline //nolint |
| [`gci`](https://github.com/daixiang0/gci) [⛭](https://golangci-lint.run/usage/linters/#gci)                                                                       | checks import order and ensures it is always deterministic       | inline //nolint |
| [`ginkgolinter`](https://github.com/nunnatsa/ginkgolinter) [⛭](https://golangci-lint.run/usage/linters/#ginkgolinter)                                             | enforces standards of using Ginkgo and Gomega                    | inline //nolint |
| [`gocheckcompilerdirectives`](https://github.com/leighmcculloch/gocheckcompilerdirectives)                                                                        | checks go compiler directive comments                            | inline //nolint |
| `gochecknoinits`                                                                                                                                                  | checks that no init functions are present                        | inline //nolint |
| [`gofmt`](https://pkg.go.dev/cmd/gofmt) [⛭](https://golangci-lint.run/usage/linters/#gofmt)                                                                       | checks whether code was [gofmt](https://pkg.go.dev/cmd/gofmt) ed |                                    |
| [`goimports`](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) [⛭](https://golangci-lint.run/usage/linters/#goimports)                                        | check import statements formatting                               | inline //nolint |
| [`gosec`](https://github.com/securego/gosec) [⛭](https://golangci-lint.run/usage/linters/#gosec)                                                                  | inspects source code for security problems                       | inline //nolint |
| [`govet`](https://pkg.go.dev/cmd/vet) [⛭](https://golangci-lint.run/usage/linters/#govet)                                                                         | examines Go source code and reports suspicious constructs        | inline //nolint |
| [`ineffassign`](https://github.com/gordonklaus/ineffassign)                                                                                                       | detects when assignments to existing variables are not used      | inline //nolint |
| [`loggercheck`](https://github.com/timonwong/loggercheck) [⛭](https://golangci-lint.run/usage/linters/#loggercheck)                                               | checks key-value pairs for common logger libraries               | inline //nolint |
| [`misspell`](https://github.com/client9/misspell) [⛭](https://golangci-lint.run/usage/linters/#misspell)                                                          | finds commonly misspelled English words in comments              | inline //nolint |
| [`nolintlint`](https://github.com/golangci/golangci-lint/blob/master/pkg/golinters/nolintlint/README.md) [⛭](https://golangci-lint.run/usage/linters/#nolintlint) | reports ill-formed or insufficient nolint directives             | inline //nolint |
| [`revive`](https://github.com/mgechev/revive) [⛭](https://golangci-lint.run/usage/linters/#revive)                                                                | comprehensive golint replacement                                 | inline //nolint |
| [`staticcheck`](https://staticcheck.io/docs/checks/) [⛭](https://golangci-lint.run/usage/linters/#staticcheck)                                                    | performs static code analysis                                    | inline //nolint |
| [`stylecheck`](https://github.com/dominikh/go-tools/tree/master/stylecheck) [⛭](https://golangci-lint.run/usage/linters/#stylecheck)                              | examines Go code-style conformance                               | inline //nolint |
| `typecheck`                                                                                                                                                       | parses and type-checks Go code                                   | inline //nolint |
| [`unparam`](https://github.com/mvdan/unparam) [⛭](https://golangci-lint.run/usage/linters/#unparam)                                                               | reports unused function parameters                               | inline //nolint |
| [`unused`](https://github.com/dominikh/go-tools/tree/master/unused)                                                                                               | checks for unused constants, variables, functions and types      | inline //nolint |

</details>

### Irrelevant Linters

The following linters are irrelevant for development of the Telemetry module:

<details>
<summary>List of irrelevant linters</summary>
<br>

| Linter             | Reason                                                               |
| ------------------ | -------------------------------------------------------------------- |
| `bidichk`          | superseded by `stylecheck`                                           |
| `deadcode`         | superseded by `unused`                                               |
| `execinquery`      | `database/sql` package is not used                                   |
| `exhaustivestruct` | superseded by `exhaustruct`                                          |
| `forcetypeassert`  | superseded by `errcheck`                                             |
| `golint`           | superseded by `revive`, `stylecheck`                                 |
| `ifshort`          | deprecated                                                           |
| `interfacer`       | deprecated                                                           |
| `maligned`         | superseded by `govet`                                                |
| `nosnakecase`      | superseded by `revive`                                               |
| `rowserrcheck`     | `database/sql` package is not used                                   |  
| `sqlclosecheck`    | `database/sql` package is not used                                   |  
| `scopelint`        | superseded by `exportloopref`                                        |
| `structcheck`      | superseded by `unused`                                               |
| `testableexamples` | Go Example functions are not used                                    |
| `varcheck`         | superseded by `unused`                                               |
| `wastedassign`     | superseded by `inefassign`                                           |

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
      path: apis/telemetry/v1alpha1/(logparsers|metricpipeline|tracepipeline)_types_test.go
```

### Dev Environment Configuration

Read [golangci-lint integrations](https://golangci-lint.run/welcome/integrations/) for information on configuring file-watchers for your IDE or code editor.

### Autofix

Some of the linting errors can be automatically fixed with the command:
`make lint_autofix`
