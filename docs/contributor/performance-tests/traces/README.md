# Traces KPIs and Limit Test

This document describes a reproducible test setup to determine the limits and KPis of the Kyma TracePipeline.

## Prerequisites

- Kyma as the target deployment environment, 2 Nodes with 4 CPU and 16G Memory (n1-standard-4 on GCP)
- Telemetry Module installed
- Istio Module installed
- Kubectl > 1.22.x
- Helm 3.x
- curl 8.4.x
- jq 1.6

## Test Cases

### Assumptions

The tests are executed for 20 minutes for each test case to have a stabilized output and reliable KPIs. Generated traces contain at least 2 spans, and each span has 40 attributes to simulate an average trace span size.  

The following test cases are identified:

1. Test average throughput end-to-end. 
2. Test queuing and retry capabilities of TracePipeline with simulated backend outages.
3. Test average throughput with 3 TracePipelines simultaneously end-to-end.
4. Test queuing and retry capabilities of 3 TracePipeline with simulated backend outages.


## Setup

The following diagram shows the test setup used for all test cases. 

![Metric gateway exported metrics](./assets/trace_perf_test_setup.drawio.svg)

In all test scenarios, a preconfigured trace load generator is deployed on the test cluster. To ensure all trace gateway instances are loaded with test data, the trace load generator feeds the test TracePipeline over a pipeline service instance .

A Prometheus instance is deployed on the test cluster to collect relevant metrics from trace gateway instances and to fetch the metrics at the end of the test as test scenario result.

All test scenarios also have a test backend deployed to simulate end-to-end behaviour.

Each test scenario has its own test scripts responsible for preparing test scenario and deploying on test cluster, running the scenario, and fetching relevant metrics/KPIs at the end of the test run. After the test, the test results are printed out.

A typical test result output looks like the following example:

```shell
 Receiver accepted spans,Average,12867.386144069678
 Exporter exported spans,Average,38585.09404079456
 Exporter queue size,Average,0
 Pod memory,telemetry-trace-collector-9fd48899-7l6f7,147464192
 Pod memory,telemetry-trace-collector-9fd48899-wdx2g,160010240
 Pod CPU,telemetry-trace-collector-9fd48899-72knt,1.4228919657370949
 Pod CPU,telemetry-trace-collector-9fd48899-7l6f7,1.414138202062809
```

## Test Script

All test scenarios use a single test script [run-load-test.sh](assets/run-load-test.sh), which provides two parameters: `-m` for multi TracePipeline scenarios, and `-b` for backpressure scenarios
1. To test the average throughput end-to-end, run:

```shell
./run-load-test.sh
```
2. To test the queuing and retry capabilities of TracePipeline with simulated backend outages, run:

```shell
./run-load-test.sh -b true
```

3. To test the average throughput with 3 TracePipelines simultaneously end-to-end, run:

```shell
./run-load-test.sh -m true
```

4. To test the queuing and retry capabilities of 3 TracePipelines with simulated backend outages, run:

```shell
./run-load-test.sh -m true -b true
```

## Test Results

|                                               Test Description |     0.91     |           0.92 |
|---------------------------------------------------------------:|:------------:|---------------:|
|                Single Pipeline - Receiver Accepted Spans / sec |   19815.05   |        21146.3 |
|                Single Pipeline - Exporter Exported Spans / sec |   19815.05   |        21146.3 |
|                          Single Pipeline - Exporter Queue Size |      0       |              0 |
|                    Single Pipeline - Pod Memory Usage (MBytes) | 137, 139.92  |   72.37, 50.95 |
|                                Single Pipeline - Pod CPU Usage | 0.979, 0.921 |   1.038, 0.926 |
|                 Multi Pipeline - Receiver Accepted Spans / sec |   13158.4    |        12757.6 |
|                 Multi Pipeline - Exporter Exported Spans / sec |   38929.06   |        38212.2 |
|                           Multi Pipeline - Exporter Queue Size |      0       |              0 |
|                     Multi Pipeline - Pod Memory Usage (MBytes) |  117, 98.5   |   90.3, 111.28 |
|                                 Multi Pipeline - Pod CPU Usage | 1.307, 1.351 |      1.36,1.19 |
| Single Pipeline - BackPressure - Receiver Accepted Spans / sec |    9574.4    |         3293.6 |
| Single Pipeline - BackPressure - Exporter Exported Spans / sec |     1280     |         2918.4 |
|           Single Pipeline - BackPressure - Exporter Queue Size |     509      |            204 |
|     Single Pipeline - BackPressure - Pod Memory Usage (MBytes) | 1929.4, 1726 |  866.07, 873.4 |
|                 Single Pipeline - BackPressure - Pod CPU Usage | 0.723, 0.702 |      0.58,0.61 |
|  Multi Pipeline - BackPressure - Receiver Accepted Spans / sec |    9663.8    |         9694.6 |
|  Multi Pipeline - BackPressure - Exporter Exported Spans / sec |    1331.2    |         1399.5 |
|            Multi Pipeline - BackPressure - Exporter Queue Size |     510      |            510 |
|      Multi Pipeline - BackPressure - Pod Memory Usage (MBytes) | 2029.8, 1686 | 1730.6, 1796.6 |
|                  Multi Pipeline - BackPressure - Pod CPU Usage | 0.733, 0.696 |   0.736, 0.728 |
                                                                                                                                                             


