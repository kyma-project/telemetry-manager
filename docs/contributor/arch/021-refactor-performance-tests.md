---
title: Refactor Performance Tests
status: Accepted
date: 2025-07-03
---

# 21. Refactor Performance Tests

## Context

Currently, [performance tests](../benchmarks/README.md) are written in bash which makes them hard to maintain. Our goal is to rewrite them in Golang so that they become easier to read, update and debug.

### POC for using components from Opentelemtry Collector Testbed

Because we are going to rewrite the performance tests in Golang, this is a good point of time to check if it is useful to use components from the [Opentelemetry Collector Testbed](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/testbed).

A [POC](../pocs/opentelemetry-testbed/) has been implemented for using the `LoadGenerator` and `MockBackend` from the Opentelemetry Collector Testbed. The performance tests in the `opentelemetry-collector-contrib` repo are executed locally on a machine (all components run as processes and not containers). In our use case, we want to test the images that eventually will run as containers in production Kubernetes clusters. So, in order to test our Telemetry module with the `LoadGenerator` and `MockBackend`, these two components are dockerized and running as pods in the POC . The setups in the POC for testing the Log Gateway and Log Agent are shown respectively in the following diagrams:

![arch](./../assets/opentelemetry-testbed-log-gateway-setup.svg)

![arch](./../assets/opentelemetry-testbed-log-agent-setup.svg)

#### Advantages:
- It is possible to determine the exact number of data items (trace spans, metric data points, or log records) sent by the load generator and the exact number of data items received by the mock backend. In the POC, these numbers are exposed as a Prometheus metric at the `/metrics` endpoint on port `2112`.

#### Disadvantages:
- There is no [DataSender](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/testbed/datasenders) available that writes logs to stdout.
    - The [FileLogWriter](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/testbed/datasenders/stanza.go) converts OTLP logs to text lines and writes them to a temporary log file. This is not suitable for our Log Agent, because it tails logs that are only written to the stdout.
    - The workaround in the POC is to write a custom `stdoutLogGenerator`, which implements the `DataSender` interface. The `stdoutLogGenerator` is very similar to the `FileLogWriter`, but it writes logs to stdout instead of a temporary log file.
- The receiver used in the mock backend is [listening](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/testbed/testbed/receivers.go#L81) on `127.0.0.1:4317` (localhost only).
    - This works fine for the performance tests in the `opentelemetry-collector-contrib` repo, because these test are executed locally. However, we must test our Telemetry module collectors in a Kubernetes cluster.
    - Because this [address](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/testbed/testbed/receivers.go#L81) is hardcoded in the receiver and not configurable, the workaround in the POC is to copy over the receiver source code and change the address to `0.0.0.0:4317` (all interfaces).
- Whenever the API for the Opentelemtry Collector Testbed changes, we must update the source code for the [loadgenerator](../pocs/opentelemetry-testbed/loadgenerator/) and the [mockbackend](../pocs/opentelemetry-testbed/mockbackend/) and rebuild their images.

#### Conclusion
The extra metric (data items sent/received) that we can get from using the components from the Opentelemtry Collector Testbed is not worth all the disadvantages mentioned above. Therefore, we are going to use the already existing components (like load generators, mock backend, ...) from our [e2e testkit](../../../test/testkit/).

## Decision

- Rewrite the existing performance tests in Golang.
- Use the already existing components from our [e2e testkit](../../../test/testkit/).
- Add tests for the new Log Gateway and Log Agent. They will have similar test cases as the existing ones for Metric Gateway and Metric Agent. The setup for testing the Log Gateway and Log Agent is shown respectively in the following diagrams:

![arch](./../assets/log-gateway-perf-test-setup.svg)

![arch](./../assets/log-agent-perf-test-setup.svg)

- For testing the Log Agent, it would be better to use a JSON log generator in which we can add custom attributes. This will allow us to test the performance of the [operators](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver#operators) we use in the filelog receiver in the Log Agent. To find the operators currently used, see the [golden test file](https://github.com/kyma-project/telemetry-manager/blob/main/internal/otelcollector/config/log/agent/testdata/config.yaml#L59) for the Log Agent.  
