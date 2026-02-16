# Development

Telemetry Manager has been bootstrapped with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) 3.6.0. To add more APIs, use [Kubebuilder](https://book.kubebuilder.io/cronjob-tutorial/new-api.html).

## Prerequisites

- Install [kubebuilder 3.6.0](https://github.com/kubernetes-sigs/kubebuilder), which is the base framework for Telemetry Manager. Required to add new APIs.
- Install [Golang 1.20](https://golang.org/dl/) or newer (for development and local execution).
- Install [Docker](https://www.docker.com/get-started/).
- Install [golangci-lint](https://golangci-lint.run).

Other dependencies will be downloaded by the make targets to the `bin` sub-folder.

## Development Commands

For development, use the following commands:

- Run `golangci-lint` and lint manifests

  ```bash
  make lint
  ```

- Autofix all automatically-fixable linter complaints

  ```bash
  make lint-fix
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
  export MANAGER_IMAGE=<my container repo>
  make docker-build
  make docker-push
  kubectl create ns kyma-system
  make deploy-experimental
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
   make run-{test-labels}
   ```

  _Example:_

   ```bash
   make run-all-e2e-logs
   make run-e2e-fluent-bit
   make run-e2e-telemetry
   ```
