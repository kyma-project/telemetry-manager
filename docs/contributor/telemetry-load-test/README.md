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

All test scenarios use a single test script [run-load-test.sh](assets/run-load-test.sh), which provides following parameters: 

- `-t` The test target type supported values are `traces, metrics, metricagent`, default is `traces`
- `-n` Test name e.g. `0.92`
- `-m` Enables multi pipeline scenarios, default is `false` 
- `-b` Enables backpressure scenarios, default is `false`

## Traces Test 

### Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. Generated traces contain at least 2 spans, and each span has 40 attributes to simulate an average trace span size.  

The following test cases are identified:

1. Test average throughput end-to-end. 
2. Test queuing and retry capabilities of TracePipeline with simulated backend outages.
3. Test average throughput with 3 TracePipelines simultaneously end-to-end.
4. Test queuing and retry capabilities of 3 TracePipeline with simulated backend outages.

Backend outages simulated with Istio Fault Injection, 70% of traffic to the Test Backend will return `HTTP 503` to simulate service outages.

### Setup

The following diagram shows the test setup used for all test cases. 

![Trace Gateway Test Setup](./assets/trace_perf_test_setup.drawio.svg)

In all test scenarios, a preconfigured trace load generator is deployed on the test cluster. To ensure all trace gateway instances are loaded with test data, the trace load generator feeds the test TracePipeline over a pipeline service instance .

A Prometheus instance is deployed on the test cluster to collect relevant metrics from trace gateway instances and to fetch the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behaviour.

Each test scenario has its own test scripts responsible for preparing test scenario and deploying on test cluster, running the scenario, and fetching relevant metrics/KPIs at the end of the test run. After the test, the test results are printed out.

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



<div class="table-wrapper" markdown="block">

| Version/Test |       Single Pipeline       |                             |                     |                      |               |       Multi Pipeline        |                             |                     |                      |               | Single Pipeline Backpressure |                             |                     |                      |               | Multi Pipeline Backpressure |                             |                     |                      |               |
|-------------:|:---------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|:---------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|:----------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|:---------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|
|              | Receiver Accepted Spans/sec | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Spans/sec | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Spans/sec  | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Spans/sec | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|         0.91 |          19815.05           |          19815.05           |          0          |     137, 139.92      | 0.979, 0.921  |           13158.4           |          38929.06           |          0          |      117, 98.5       | 1.307, 1.351  |            9574.4            |            1280             |         509         |     1929.4, 1726     | 0.723, 0.702  |           9663.8            |           1331.2            |         510         |     2029.8, 1686     | 0.733, 0.696  |
|         0.92 |           21146.3           |           21146.3           |          0          |     72.37, 50.95     | 1.038, 0.926  |           12757.6           |           38212.2           |          0          |     90.3, 111.28     |  1.36, 1.19   |            3293.6            |           2918.4            |         204         |    866.07, 873.4     |  0.58, 0.61   |           9694.6            |           1399.5            |         510         |    1730.6, 1796.6    | 0.736, 0.728  |

</div>


## Metrics Test

Metric test consists of two main test scenarios. The first scenario tests the Metric Gateway KPIs, and the second one tests Metric Agent KPIs.

### Metric Gateway Test and Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. Generated metrics contain 10 attributes to simulate an average metric size; the test simulates 2000 individual metrics producers, and each one pushes metrics every 30 second to the Metric Gateway.


The following test cases are identified:

1. Test average throughput end-to-end.
2. Test queuing and retry capabilities of Metric Gateway with simulated backend outages.
3. Test average throughput with 3 MetricPipelines simultaneously end-to-end.
4. Test queuing and retry capabilities of 3 MetricPipeline with simulated backend outages.

Backend outages are simulated with Istio Fault Injection: 70% of the traffic to the test backend will return `HTTP 503` to simulate service outages.

### Metric Agent Test and Assumptions

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. 
In contrast to the Metric Gateway test, the Metric Agent test deploys a passive metric producer ([Avalanche Prometheus metric load generator](https://blog.freshtracks.io/load-testing-prometheus-metric-ingestion-5b878711711c)) and the metrics are scraped by Metric Agent from the producer.
The test setup deploys 20 individual metric producer Pods; each which produces 1000 metrics with 10 metric series. To test both Metric Agent receiver configurations, Metric Agent collects metrics with Pod scraping as well as Service scraping.


The following test cases are identified:

1. Test average throughput end-to-end.
2. Test queuing and retry capabilities of Metric Agent with simulated backend outages.

Backend outages simulated with Istio Fault Injection, 70% of traffic to the Test Backend will return `HTTP 503` to simulate service outages

### Setup

The following diagram shows the test setup used for all Metric test cases.

![Metric Test Setup](./assets/metric_perf_test_setup.drawio.svg)


In all test scenarios, a preconfigured trace load generator is deployed on the test cluster. To ensure all Metric Gateway instances are loaded with test data, the trace load generator feeds the test MetricPipeline over a pipeline service instance, in Metric Agent test, test data scraped from test data producer and pushed to the Metric Gateway. 

A Prometheus instance is deployed on the test cluster to collect relevant metrics from Metric Gateway and Metric Agent instances and to fetch the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behaviour.

Each test scenario has its own test scripts responsible for preparing test scenario and deploying on test cluster, running the scenario, and fetching relevant metrics/KPIs at the end of the test run. After the test, the test results are printed out.

### Running Tests

#### Metric Gateway

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

#### Test Results



<div class="table-wrapper" markdown="block">

| Version/Test |       Single Pipeline        |                              |                     |                      |               |        Multi Pipeline        |                              |                     |                      |               | Single Pipeline Backpressure |                              |                     |                      |               | Multi Pipeline Backpressure  |                              |                     |                      |               |
|-------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|              | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|         0.92 |             5992             |             5993             |          0          |       225, 178       |   1.6, 1.5    |             4882             |            14647             |          0          |       165, 255       |   1.7, 1.8    |             635              |             636              |         114         |       770, 707       |     0, 0      |             965              |             1910             |         400         |      1694, 1500      |   0.1, 0.1    |

</div>

#### Metric Agent

1. To test the average throughput end-to-end, run:

```shell
./run-load-test.sh -t metricagent -n "0.92"
```
2. To test the queuing and retry capabilities of Metric Agent with simulated backend outages, run:

```shell
./run-load-test.sh -t metricagent -n "0.92" -b true
```

#### Test Results



<div class="table-wrapper" markdown="block">

| Version/Test |       Single Pipeline        |                              |                     |                      |               | Single Pipeline Backpressure |                              |                     |                      |               |
|-------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|              | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|         0.92 |            20123             |            20137             |          0          |       704, 747       |   0.2, 0.2    |            19952             |            15234             |          0          |       751, 736       |   0.3, 0.2    |

</div>