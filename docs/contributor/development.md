# Development

Telemetry Manager has been bootstrapped with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) 3.6.0. Too add more APIs, use [Kubebuilder](https://book.kubebuilder.io/cronjob-tutorial/new-api.html).

## Prerequisites

- Install [kubebuilder 3.6.0](https://github.com/kubernetes-sigs/kubebuilder), which is the base framework for Telemetry Manager. Required to add new APIs.
- Install [Golang 1.20](https://golang.org/dl/) or newer (for development and local execution).
- Install [Docker](https://www.docker.com/get-started).
- Install [golangci-lint](https://golangci-lint.run).
- Install [ginkgo CLI](https://pkg.go.dev/github.com/onsi/ginkgo/ginkgo) for running the E2E test commands straight from your terminal.

Other dependencies will be downloaded by the make targets to the `bin` sub-folder.

## Development Commands

For development, use the following commands:

- Run `golangci-lint` and lint manifests

  ```bash
  make lint
  ```

- Autofix all automatically-fixable linter complaints

  ```bash
  make lint-autofix
  ```

- Regenerate YAML manifests (CRDs and RBAC)

  ```bash
  make manifests
  ```

- Install CRDs to cluster in current kubeconfig context

  ```bash
  make install
  ```

- Uninstall CRDs to cluster in current kubeconfig context

  ```bash
  make uninstall
  ```

- Run Telemetry Manager locally (uses current kubeconfig context)

  ```bash
  make run
  ```

- Build container image and deploy to cluster in current kubeconfig context

  ```bash
  export IMG=<my container repo>
  make docker-build
  make docker-push
  kubectl create ns kyma-system
  make deploy-dev
  ```

- Clean up everything

  ```bash
  make undeploy
  ```

## Testing Commands

For testing, use the following commands:

- Run unit tests

  ```bash
  make test
  ```

- Deploy module with Lifecycle Manager on a k3d cluster

  ```bash
  make provision-k3d-e2e
  ```

- Run e2e tests
  ```bash
  export IMG=<my container repo> make <make deploy target>
  ginkgo run --tags e2e --junit-report=<report output location> --label-filter="<e2e filter>" ./test/e2e
  ```
  _Examples:_
  - ```bash
    export IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy-dev
    ginkgo run --tags e2e --junit-report=./artifacts/junit.xml --label-filter="logs" ./test/e2e
    ```
  - ```bash
    export IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
    ginkgo run --tags e2e --junit-report=./artifacts/junit.xml --label-filter="traces" ./test/e2e
    ```
- Run tests using `hack/run-tests.sh`
  ```bash
  hack/run-tests.sh <type> <test suite>
  ```
  _Examples:_
  - ```bash
    hack/run-tests.sh e2e metrics
    hack/run-tests.sh integration istio
    hack/run-tests.sh upgrade operational
    ```
