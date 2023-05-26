# Telemetry Manager

## Overview

Telemetry Manager is a Kubernetes operator which is fullfiling the [Kyma module interface](https://github.com/kyma-project/community/tree/main/concepts/modularization). It is providing APIs for a managed agent/gateway setup for log, trace, and metric ingestion and dispatching into 3party backend systems, in order to reduce the pain of orchestrating usch setup on your own. Read more on the [usage](./docs/user/README.md) of the module as well as general [design and strategy](https://github.com/kyma-project/community/blob/main/concepts/observability-strategy/strategy.md) behind the module.

### Logs

The logging controllers generate a Fluent Bit DaemonSet and configuration from one or more LogPipeline and LogParser custom resources. The controllers ensure that all Fluent Bit Pods run the current configuration by restarting Pods after the configuration has changed. See all [CRD attributes](apis/telemetry/v1alpha1/logpipeline_types.go) and some [examples](config/samples).

Further design decisions and test results are documented in [Dynamic Logging Backend Configuration](https://github.com/kyma-project/community/tree/main/concepts/observability-strategy/configurable-logging).

### Traces

The trace controller creates an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) deployment and related Kubernetes objects from a `TracePipeline` custom resource. The collector is configured to receive traces using the OTLP and OpenCensus protocols, and forwards the received traces to a configurable OTLP backend.

See [Dynamic Trace Backend Configuration](https://github.com/kyma-project/community/tree/main/concepts/observability-strategy/configurable-tracing) for further information.

### Metrics

The metric controller creates an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) and related Kubernetes objects from a `MetricPipeline` custom resource. The collector is deployed as a [Gateway](https://opentelemetry.io/docs/collector/deployment/#gateway). The controller is configured to receive metrics in the OTLP protocol and forward them to a configurable OTLP backend.

See [Dynamic Monitoring Backend Configuration](https://github.com/kyma-project/community/tree/main/concepts/observability-strategy/configurable-monitoring) for further information.

## Usage

More information can be found at the dedicated [usage documentation](./docs/user/README.md)

## Installation

More information can be found in the dedicated [instruction](./docs/contributor/installation.md).

## Development

More information can be found in dedicated documents:
- [Available commands for building/linting/installation](./docs/contributor/development.md)
- [Testing Strategy](./docs/contributor/testing.md)
- [Troubleshooting and Debugging](./docs/contributor/troubleshooting.md)
- [Release process](./docs/contributor/releasing.md)
- [Governance checks like linting](./docs/contributor/governance.md)

## License

This project is licensed under the Apache Software License, version 2.0 except as noted otherwise in the [LICENSE](./LICENSE) file.

## Contributing

To contribute to this project, follow the general [Kyma project contributing](https://github.com/kyma-project/community/blob/main/docs/contributing/02-contributing.md) guidelines.
