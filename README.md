# Telemetry Manager

## Overview

To implement [Kyma's strategy](https://github.com/kyma-project/community/blob/main/concepts/observability-strategy/strategy.md) of moving from in-cluster observability backends to a Telemetry component that integrates with external backends, Telemetry Manager is a Kubernetes operator that provides APIs for configurable logging, tracing, and monitoring.

Telemetry Manager has been bootstrapped with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) 3.6.0. Additional APIs can also be [added by Kubebuilder](https://book.kubebuilder.io/cronjob-tutorial/new-api.html).

### Configurable Logging

The logging controllers generate a Fluent Bit DaemonSet and configuration from one or more LogPipeline and LogParser custom resources. The controllers ensure that all Fluent Bit Pods run the current configuration by restarting Pods after the configuration has changed. See all [CRD attributes](apis/telemetry/v1alpha1/logpipeline_types.go) and some [examples](config/samples).

Further design decisions and test results are documented in [Dynamic Logging Backend Configuration](https://github.com/kyma-project/community/tree/main/concepts/observability-strategy/configurable-logging).

### Configurable Tracing

The trace controller creates an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) deployment and related Kubernetes objects from a `TracePipeline` custom resource. The collector is configured to receive traces using the OTLP and OpenCensus protocols, and forwards the received traces to a configurable OTLP backend.

See [Dynamic Trace Backend Configuration](https://github.com/kyma-project/community/tree/main/concepts/observability-strategy/configurable-tracing) for further information.

### Configurable Monitoring

The metric controller creates an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) and related Kubernetes objects from a `MetricPipeline` custom resource. The collector is deployed as a [Gateway](https://opentelemetry.io/docs/collector/deployment/#gateway). The controller is configured to receive metrics in the OTLP protocol and forward them to a configurable OTLP backend.

See [Dynamic Monitoring Backend Configuration](https://github.com/kyma-project/community/tree/main/concepts/observability-strategy/configurable-monitoring) for further information.

## Development

### Prerequisites

- Install [kubebuilder 3.6.0](https://github.com/kubernetes-sigs/kubebuilder), which is the base framework for this controller. Required to add new APIs.
- Install [Golang 1.19](https://golang.org/dl/) or newer (for development and local execution).
- Install [Docker](https://www.docker.com/get-started).
- Install [golangci-lint](https://golangci-lint.run).

Other dependencies will be downloaded by the make targets to the `bin` sub-folder.

### Available Commands

For development, you can use the following commands:

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

- Run the operator locally (uses current kubeconfig context)

```bash
make run
```

- Build container image and deploy to cluster in current kubeconfig context

```bash
export IMG=<my container repo>
make docker-build
make docker-push
kubectl create ns kyma-system
make deploy
```

- Clean up everything

```bash
make undeploy
```

### Deploying `ModuleTemplate` with the Lifecycle Manager
Check the documentation [here](./docs/deploying-module-template.md).

## Troubleshooting

### Enable pausing reconciliations

You must pause reconciliations to be able to debug the pipelines and, for example, try out a different pipeline configuration or a different OTel configuration. To pause reconciliations, create a `telemetry-override-config` in the operators Namespace.
Here is an example of such a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: telemetry-override-config
data:
  override-config: |
    global:
      logLevel: debug
    tracing:
      paused: true
    logging:
      paused: true
    metrics:
      paused: true
```

The `global`, `tracing`, `logging` and `metrics` fields are optional.

#### Debugging steps

1. Create an overriding `telemetry-override-config` ConfigMap.
2. Perform debugging operations.
3. Remove the created ConfigMap.
4. To reset the debug actions, perform a restart of Telemetry Manager.

   ```bash
   kubectl rollout restart deployment telemetry-controller-manager
   ```

**Caveats**
If you change the pipeline CR when the reconciliation is paused, these changes will not be applied immediately but in a periodic reconciliation cycle of one hour. To reconcile earlier, restart Telemetry Manager.

### Profiling

Telemetry Manager has pprof-based profiling activated and exposed on port 6060. Use port-forwarding to access the pprof endpoint. You can find additional information in the Go [pprof package documentation](https://pkg.go.dev/net/http/pprof).

### MackBook M1 users

For MacBook M1 users, some parts of the scripts may not work and they might see an error message like the following:
`Error: unsupported platform OS_TYPE: Darwin, OS_ARCH: arm64; to mitigate this problem set variable KYMA with the absolute path to kyma-cli binary compatible with your operating system and architecture. Stop.`

That's because Kyma CLI is not released for Apple Silicon users. 

To fix it, install [Kyma CLI manually](https://github.com/kyma-project/cli#installation) and export the path to it.

   ```bash
   export KYMA=$(which kyma)
   ```

