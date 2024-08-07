# Load Test for logs using OtelCollector

This document describes a reproducible test setup to determine the performance of a gateway based on OpenTelemetry

## Prerequisites

- 2 Nodes with 4 CPU and 16G Memory (n1-standard-4 on GCP)
- Kubectl > 1.22.x
- Helm 3.x
- curl 8.4.x
- jq 1.6


## Test Script

The test scenario is implemented in the existing bash script [run-load-test.sh](../../../hack/load-tests/run-load-test.sh).
It can be invoked with the following parameters:

- `-t logs-otel`
- `-n` Test name
- `-d` The test duration in second, default is `1200` seconds
- `-r` The rate of log generation in log/s, default is `1000` logs/s

### Setup

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. Generated logs
contain one single log line of ~2000 bytes.

A typical test result output looks like the following example:


### Test Results

<div class="table-wrapper" markdown="block">

|         | Receiver Accepted logs/sec | Exporter Exported logs/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|---------|----------------------------|----------------------------|---------------------|----------------------|---------------|
| 0.105.0 | 5992                       | 5993                       | 0                   | 225                  | 1.6           |


|   test_name   |test_target|overlay|max_pipeline|               nodes              |backpressure_test|test_duration|EXPORTED|RESTARTS_GATEWAY|CPU|RECEIVED|QUEUE|TYPE|MEMORY|RESTARTS_GENERATOR| mode |
|---------------|-----------|-------|------------|----------------------------------|-----------------|-------------|--------|----------------|---|--------|-----|----|------|------------------|------|
|   logs-otel   | logs-otel | batch |    false   |['n1-standard-4', 'n1-standard-4']|      false      |     1200    |  7117  |        0       |2.5|  7115  |11716| log|  772 |         6        | batch|
|logs-otel-batch| logs-otel |       |    false   |['n1-standard-4', 'n1-standard-4']|      false      |     1200    |  6353  |        0       |2.4|  6349  |13404| log|  809 |         2        |single|
</div>

## Interpretation

The test results show that the gateway is able to process 5992 logs/sec. The memory usage is 225MB and the CPU usage is 1.6. A CLS instance (Plan: Enterprise) is able to process 10_000 logs/sec (size: 2k/log).
In order to achieve this, the gateway must be scaled horizontally. Two instances of the gateway are required to process more than 10_000 logs/sec. It is essential to enable the batch processor to achieve the desired performance.

The downside of the batch processor is that - if used with the grpc protocol - the maximum batch size needs to be configured. With too many log entries per batch the maximum message size (4MB by default) can be exceeded. This can lead to a loss of log entries.
The maximum number of log entries per batch highly depends on the average log size. For the tests a safe maximum of 500 logs per batch was used.
