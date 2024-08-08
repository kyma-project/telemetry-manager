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

</div>
