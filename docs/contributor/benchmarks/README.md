# Telemetry KPIs and Limit Test

This document describes a reproducible test setup to determine the limits and KPis of the Kyma TracePipeline and MetricPipeline.

## Prerequisites

- Kyma as the target deployment environment, 2 Nodes with 4 CPU and 16G Memory (n1-standard-4 on GCP)
- Telemetry Module installed
- Istio Module installed
- Kubectl > 1.22.x
- Helm 3.x
- curl 8.4.x
- jq 1.6

## Test Script

All test scenarios use a single test script [run-load-test.sh](../../../hack/load-tests/run-load-test.sh), which
provides following parameters:

- `-t` The test target type supported values are `traces, metrics, metricagent, logs-fluentbit, self-monitor`, default
  is `traces`
- `-n` Test name e.g. `0.92`
- `-m` Enables multi pipeline scenarios, default is `false`
- `-b` Enables backpressure scenarios, default is `false`
- `-d` The test duration in second, default is `1200` seconds

## Traces Test

### Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. Generated
traces contain at least 2 spans, and each span has 40 attributes to simulate an average trace span size.

The following test cases are identified:

- Test average throughput end-to-end.
- Test queuing and retry capabilities of TracePipeline with simulated backend outages.
- Test average throughput with 3 TracePipelines simultaneously end-to-end.
- Test queuing and retry capabilities of 3 TracePipeline with simulated backend outages.

Backend outages simulated with Istio Fault Injection, 70% of traffic to the Test Backend will return `HTTP 503` to
simulate service outages.

### Setup

The following diagram shows the test setup used for all test cases.

![Trace Gateway Test Setup](./assets/trace_perf_test_setup.drawio.svg)

In all test scenarios, a preconfigured trace load generator is deployed on the test cluster. To ensure all trace gateway
instances are loaded with test data, the trace load generator feeds the test TracePipeline over a pipeline service
instance .

A Prometheus instance is deployed on the test cluster to collect relevant metrics from trace gateway instances and to
fetch the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behaviour.

Each test scenario has its own test scripts responsible for preparing test scenario and deploying on test cluster,
running the scenario, and fetching relevant metrics/KPIs at the end of the test run. After the test, the test results
are printed out.

A typical test result output looks like the following example:

```shell
|          |Receiver Accepted Span/sec  |Exporter Exported Span/sec  |Exporter Queue Size |    Pod Memory Usage(MB)    |    Pod CPU Usage     |
|   0.92   |           5992             |           5993             |           0        |        225, 178            |        1.6, 1.5      |
```

### Running Tests

1. To test the average throughput end-to-end, run:

```shell
./run-load-test.sh -t traces -n "0.92"
```

2. To test the queuing and retry capabilities of TracePipeline with simulated backend outages, run:

```shell
./run-load-test.sh -t traces -n "0.92" -b true
```

3. To test the average throughput with 3 TracePipelines simultaneously end-to-end, run:

```shell
./run-load-test.sh -t traces -n "0.92" -m true
```

4. To test the queuing and retry capabilities of 3 TracePipelines with simulated backend outages, run:

```shell
./run-load-test.sh -t traces -n "0.92" -m true -b true
```

### Test Results

For more information, see [Test Results: Traces](./results/traces.md).

## Metrics Test

The metrics test consists of two main test scenarios. The first scenario tests the Metric Gateway KPIs, and the second
one tests Metric Agent KPIs.

### Metric Gateway Test and Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. Generated
metrics contain 10 attributes to simulate an average metric size; the test simulates 2000 individual metrics producers,
and each one pushes metrics every 30 second to the Metric Gateway.

The following test cases are identified:

- Test average throughput end-to-end.
- Test queuing and retry capabilities of Metric Gateway with simulated backend outages.
- Test average throughput with 3 MetricPipelines simultaneously end-to-end.
- Test queuing and retry capabilities of 3 MetricPipeline with simulated backend outages.

Backend outages are simulated with Istio Fault Injection: 70% of the traffic to the test backend will return `HTTP 503`
to simulate service outages.

### Metric Agent Test and Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs.
In contrast to the Metric Gateway test, the Metric Agent test deploys a passive metric
producer ([Avalanche Prometheus metric load generator](https://blog.freshtracks.io/load-testing-prometheus-metric-ingestion-5b878711711c))
and the metrics are scraped by Metric Agent from the producer.
The test setup deploys 20 individual metric producer Pods; each which produces 1000 metrics with 10 metric series. To
test both Metric Agent receiver configurations, Metric Agent collects metrics with Pod scraping as well as Service
scraping.

The following test cases are identified:

- Test average throughput end-to-end.
- Test queuing and retry capabilities of Metric Agent with simulated backend outages.

Backend outages simulated with Istio Fault Injection, 70% of traffic to the Test Backend will return `HTTP 503` to
simulate service outages

### Setup

The following diagram shows the test setup used for all Metric test cases.

![Metric Test Setup](./assets/metric_perf_test_setup.drawio.svg)

In all test scenarios, a preconfigured trace load generator is deployed on the test cluster. To ensure all Metric
Gateway instances are loaded with test data, the trace load generator feeds the test MetricPipeline over a pipeline
service instance, in Metric Agent test, test data scraped from test data producer and pushed to the Metric Gateway.

A Prometheus instance is deployed on the test cluster to collect relevant metrics from Metric Gateway and Metric Agent
instances and to fetch the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behaviour.

Each test scenario has its own test scripts responsible for preparing test scenario and deploying on test cluster,
running the scenario, and fetching relevant metrics/KPIs at the end of the test run. After the test, the test results
are printed out.

### Running Gateway Tests

1. To test the average throughput end-to-end, run:

```shell
./run-load-test.sh -t metrics -n "0.92"
```

2. To test the queuing and retry capabilities of Metric Gateway with simulated backend outages, run:

```shell
./run-load-test.sh -t metrics -n "0.92" -b true
```

3. To test the average throughput with 3 TracePipelines simultaneously end-to-end, run:

```shell
./run-load-test.sh -t metrics -n "0.92" -m true
```

4. To test the queuing and retry capabilities of 3 TracePipelines with simulated backend outages, run:

```shell
./run-load-test.sh -t metrics -n "0.92" -m true -b true
```

### Running Metric Agent Tests

1. To test the average throughput end-to-end, run:

```shell
./run-load-test.sh -t metricagent -n "0.92"
```

2. To test the queuing and retry capabilities of Metric Agent with simulated backend outages, run:

```shell
./run-load-test.sh -t metricagent -n "0.92" -b true
```

### Test Results

For more information, see [Test Results: Metrics](./results/metrics.md).

## Log Test (Fluent-Bit)

### Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs.
The Log test deploys a passive log producer ([Flog](https://github.com/mingrammer/flog)), and the logs are collected by
Fluent Bit from each producer instance.
The test setup deploys 20 individual log producer Pods; each of which produces ~10 MByte logs.

The following test cases are identified:

- Test average throughput end-to-end.
- Test buffering and retry capabilities of LogPipeline with simulated backend outages.
- Test average throughput with 3 LogPipelines simultaneously end-to-end.
- Test buffering and retry capabilities of 3 LogPipeline with simulated backend outages.

Backend outages are simulated with Istio Fault Injection, 70% of traffic to the test backend will return `HTTP 503` to
simulate service outages.

### Setup

The following diagram shows the test setup used for all test cases.

![LogPipeline Test Setup](./assets/log_perf_test_setup.drawio.svg)

In all test scenarios, a preconfigured trace load generator is deployed on the test cluster.

A Prometheus instance is deployed on the test cluster to collect relevant metrics from Fluent Bit instances and to fetch
the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behaviour.

Each test scenario has its own test scripts responsible for preparing the test scenario and deploying it on the test
cluster, running the scenario, and fetching relevant metrics and KPIs at the end of the test run. After the test, the
test results are printed out.

### Running Tests

1. To test the average throughput end-to-end, run:

```shell
./run-load-test.sh -t logs-fluentbit -n "2.2.1"
```

2. To test the buffering and retry capabilities of LogPipeline with simulated backend outages, run:

```shell
./run-load-test.sh -t logs-fluentbit -n "2.2.1" -b true
```

3. To test the average throughput with 3 LogPipelines simultaneously end-to-end, run:

```shell
./run-load-test.sh -t logs-fluentbit -n "2.2.1" -m true
```

4. To test the buffering and retry capabilities of 3 LogPipelines with simulated backend outages, run:

```shell
./run-load-test.sh -t logs-fluentbit -n "2.2.1" -m true -b true
```

### Test Results

For more information, see [Test Results: Logs](./results/logs.md).

## Self Monitor

### Assumptions

The test is executed for 20 minutes. In this test case, 3 LogPipelines, 3 MetricPipelines with mode, and 3
TracePipelines with backpressure simulation are deployed on the test cluster.
Each pipeline instance is loaded with synthetic load to ensure all possible metrics are generated and collected by Self
Monitor.

Backend outages are simulated with Istio Fault Injection, 70% of traffic to the test backend will return `HTTP 503` to
simulate service outages.

### Setup

The following diagram shows the test setup.

![Self Monitor Test Setup](./assets/selfmonitor_perf_test_setup.drawio.svg)

In this test scenario, a preconfigured load generator is deployed on the test cluster.

A Prometheus instance is deployed on the test cluster to collect relevant metrics from the Self Monitor instance and to
fetch the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behavior.

This test measures the ingestion rate and resource usage of Self Monitor. The measured ingestion rate is based on
pipelines deployed with this test case with 4 Trace Gateway, 4 Metric Gateway, 2 Fluent Bit, and 2 Metric Agent Pods.

The average measured values with these 12 target Pods in total, must be the following:

- Scrape Samples/sec: 15 - 22 samples/sec
- Total Series Created: 200 - 350 series

Configured memory, CPU limits, and storage are based on this base value and will work up to max scrape 120 targets.

### Running Tests

1. To test the average throughput of Self Monitor, run:

```shell
./run-load-test.sh -t self-monitor -n "2.45.5"
```

### Test Results

For more information, see [Test Results: Self-Monitor](./results/self-monitor.md).
