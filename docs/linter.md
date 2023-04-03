# Sourcecode linting

We use [golangci-lint](https://golangci-lint.run) with fine-grained configuration for the source code linting.

## Linters in action

The following linters are configured and integrated as a CI stage through a [ProwJob](https://github.com/kyma-project/test-infra/blob/main/prow/jobs/kyma/components/kyma-components-static-checks.yaml#L6).

| Linter                                                                                                                                                            | Description                                                      | Suppress                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------| ---------------------------------------------------------------- | ---------------------------------- |
| [`asasalint`](https://github.com/alingse/asasalint) [⛭](https://golangci-lint.run/usage/linters/#asasalint)                                                       | check for pass []any as any in variadic func                     | [inline //nolint](#inline-nolint) |
| [`asciicheck`](https://github.com/tdakkota/asciicheck)                                                                                                            | checks for non-ASCII identifiers                                 | [inline //nolint](#inline-nolint) |
| [`bodyclose`](https://github.com/timakin/bodyclose)                                                                                                               | checks whether HTTP response body is closed successfully         | [inline //nolint](#inline-nolint) |
| [`dogsled`](https://github.com/alexkohler/dogsled) [⛭](https://golangci-lint.run/usage/linters/#dogsled)                                                          | checks assignments with too many blank identifiers               | [inline //nolint](#inline-nolint) |
| [`dupl`](https://github.com/mibk/dupl) [⛭](https://golangci-lint.run/usage/linters/#dupl)                                                                         | checks for code clone detection                                  |                                    |
| [`dupword`](https://github.com/Abirdcfly/dupword) [⛭](https://golangci-lint.run/usage/linters/#dupword)                                                           | checks for duplicate words in the source code                    | [inline //nolint](#inline-nolint) |
| [`errcheck`](https://github.com/kisielk/errcheck) [⛭](https://golangci-lint.run/usage/linters/#errcheck)                                                          | checks for unhandled errors                                      | [inline //nolint](#inline-nolint) |
| [`errchkjson`](https://github.com/breml/errchkjson) [⛭](https://golangci-lint.run/usage/linters/#errchkjson)                                                      | checks types passed to the json encoding functions               | [inline //nolint](#inline-nolint) |
| `exportloopref`                                                                                                                                                   | finds exporting pointers for loop variables                      | [inline //nolint](#inline-nolint) |
| [`gci`](https://github.com/daixiang0/gci) [⛭](https://golangci-lint.run/usage/linters/#gci)                                                                       | checks import order and ensures it is always deterministic       | [inline //nolint](#inline-nolint) |
| [`ginkgolinter`](https://github.com/nunnatsa/ginkgolinter) [⛭](https://golangci-lint.run/usage/linters/#ginkgolinter)                                             | enforces standards of using Ginkgo and Gomega                    | [inline //nolint](#inline-nolint) |
| [`gocheckcompilerdirectives`](https://github.com/leighmcculloch/gocheckcompilerdirectives)                                                                        | checks go compiler directive comments                            | [inline //nolint](#inline-nolint) |
| `gochecknoinits`                                                                                                                                                  | checks that no init functions are present                        | [inline //nolint](#inline-nolint) |
| [`gofmt`](https://pkg.go.dev/cmd/gofmt) [⛭](https://golangci-lint.run/usage/linters/#gofmt)                                                                       | checks whether code was [gofmt](https://pkg.go.dev/cmd/gofmt) ed |                                    |
| [`goimports`](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) [⛭](https://golangci-lint.run/usage/linters/#goimports)                                        | check import statements formatting                               | [inline //nolint](#inline-nolint) |
| [`gosec`](https://github.com/securego/gosec) [⛭](https://golangci-lint.run/usage/linters/#gosec)                                                                  | inspects source code for security problems                       | [inline //nolint](#inline-nolint) |
| [`govet`](https://pkg.go.dev/cmd/vet) [⛭](https://golangci-lint.run/usage/linters/#govet)                                                                         | examines Go source code and reports suspicious constructs        | [inline //nolint](#inline-nolint) |
| [`ineffassign`](https://github.com/gordonklaus/ineffassign)                                                                                                       | detects when assignments to existing variables are not used      | [inline //nolint](#inline-nolint) |
| [`loggercheck`](https://github.com/timonwong/loggercheck) [⛭](https://golangci-lint.run/usage/linters/#loggercheck)                                               | checks key-value pairs for common logger libraries               | [inline //nolint](#inline-nolint) |
| [`misspell`](https://github.com/client9/misspell) [⛭](https://golangci-lint.run/usage/linters/#misspell)                                                          | finds commonly misspelled English words in comments              | [inline //nolint](#inline-nolint) |
| [`nolintlint`](https://github.com/golangci/golangci-lint/blob/master/pkg/golinters/nolintlint/README.md) [⛭](https://golangci-lint.run/usage/linters/#nolintlint) | reports ill-formed or insufficient nolint directives             | [inline //nolint](#inline-nolint) |
| [`revive`](https://github.com/mgechev/revive) [⛭](https://golangci-lint.run/usage/linters/#revive)                                                                | comprehensive golint replacement                                 | [inline //nolint](#inline-nolint) |
| [`staticcheck`](https://staticcheck.io/docs/checks/) [⛭](https://golangci-lint.run/usage/linters/#staticcheck)                                                    | performs static code analysis                                    | [inline //nolint](#inline-nolint) |
| [`stylecheck`](https://github.com/dominikh/go-tools/tree/master/stylecheck) [⛭](https://golangci-lint.run/usage/linters/#stylecheck)                              | examines Go code-style conformance                               | [inline //nolint](#inline-nolint) |
| `typecheck`                                                                                                                                                       | parses and type-checks Go code                                   | [inline //nolint](#inline-nolint) |
| [`unparam`](https://github.com/mvdan/unparam) [⛭](https://golangci-lint.run/usage/linters/#unparam)                                                               | reports unused function parameters                               | [inline //nolint](#inline-nolint) |
| [`unused`](https://github.com/dominikh/go-tools/tree/master/unused)                                                                                               | checks for unused constants, variables, functions and types      | [inline //nolint](#inline-nolint) |

## Irrelevant linters

Below is the list of linters irrelevant for Huskies team.

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

## Nolint
Some linters produce false-positive warnings, so there is a way to suppress them. Consider disabling the linter if linting warnings are routinely ignored or adding to development noise.

### Inline //nolint
To suppress a linting warning for a particular line of code, use nolint instruction `//no-lint:{LINTER} // {COMMENT}.` _LINTER_ and _COMMENT_ are two mandatory placeholders with the linter to be suppressed and a reason for the suppression.


Preceding inline nolint comments for the code blocks suppress linting warnings for the whole block. The following example suppresses the linter for the entire file:
```go
//nolint:errcheck // The error check should be suppressied for the module.
package config

// The rest of the file will not be linted by errcheck.
```

### Lining exclusions
To prevent some files from being linted, there is a `.issues.exclude-rules` section in the `.golangci.yaml` configuration file. 

The code duplication linting scenario is problematic for being disabled an a per line basis. So:
```yaml
issues:
  exclude-rules:
    - linters: [ dupl ]
      path: apis/telemetry/v1alpha1/(logparsers|metricpipeline|tracepipeline)_types_test.go
```
suppresses the `dupl` linter for the set of three files in the _apis/telemetry/v1alpha1_ module.

The benefit of declaring linter exclusion rules on a file basis in the config file and not as package-level inline nolint instructions is that it is easier to visualize and analyse the linting suppressions.

# Dev environment configuration
Read [golangci-lint integrations](https://golangci-lint.run/usage/integrations/) for information on configuring file-watchers for your IDE or code editor.

## Autofix
Some of the linting errors can be automatically fixed with the command:
`make lint_autofix`
