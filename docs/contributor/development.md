# Development

Telemetry Manager has been bootstrapped with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) 3.6.0. 
Too add more APIs, use [Kubebuilder](https://book.kubebuilder.io/cronjob-tutorial/new-api.html).

## Prerequisites

- Install [kubebuilder 3.6.0](https://github.com/kubernetes-sigs/kubebuilder), which is the base framework for Telemetry Manager. Required to add new APIs.
- Install [Golang 1.20](https://golang.org/dl/) or newer (for development and local execution).
- Install [Docker](https://www.docker.com/get-started).
- Install [golangci-lint](https://golangci-lint.run).

Other dependencies will be downloaded by the make targets to the `bin` sub-folder.

## Available Commands

For development, use the following commands:

- Run unit tests
  ```bash
  make test
  ```

- Create a k3d cluster on Docker, deploy Telemetry Manager, and run integration tests
  ```bash
  make e2e-test
  ```

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

- Deploy module with Lifecycle Manager on a k3d cluster
  ```bash
  make provision-k3d-e2e
  ```
