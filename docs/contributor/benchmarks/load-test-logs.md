# Load Test for logs using OtelCollector

This document describes a reproducible test setup to determine the performance of a gateway based on OpenTelemetry

## Infratructure Pre-requisites

- Kubernetes cluster with 2 Nodes with 4 CPU and 16G Memory (n1-standard-4 on GCP)
- Kubectl > 1.22.x
- Helm 3.x
- curl 8.4.x
- jq 1.6


## Test Script

The test scenario is implemented in the existing bash script [run-load-test.sh](../../../hack/load-tests/run-load-test.sh).
Invoke it with the following parameters:

- `-t logs-otel`
- `-n` Test name
- `-d` The test duration in seconds, default is `1200` seconds
- `-r` The rate of log generation in log/s, default is `1000` logs/s

### Setup

Multiple instances of the `telemetry-gen` tool run and send logs to the `log-gateway` service. The `log-gateway` sends all incoming logs (using the configured pipeline) to the `log-receiver` service.
`log-receiver` is another instance of an OTel Collector configured to accept as many log entries as possible and directly discard them using the `nop` exporter.

The tests are executed for 20 minutes, so that each test case has a stabilized output and reliable KPIs. Logs generated by the `log-generator` contain one single log line of ~2000 bytes.


## Test Results

<div class="table-wrapper" markdown="block">

| config | logs received l/s | logs exported l/s | logs queued | cpu | memory MB | no. restarts of gateway | no. restarts of generator |
| ------ | ----------------- | ----------------- | ----------- | --- | --------- | ----------------------- | ------------------------- |
| single | 7193              | 7195              | 16824       | 2.5 | 826       | 0                       | 1                         |
| batch  | 16428             | 16427             | 0           | 3   | 265       | 0                       | 1                         |
</div>

## Interpretation

The results clearly show the beneficial impact of using the batch processor in the gateways pipeline. The batch processor can handle more than twice the amount of logs compared to the single processor. The CPU usage is slightly higher, but the memory usage is significantly lower. The number of logs queued is also significantly lower, which indicates that the batch processor is able to keep up with the incoming logs.

Similar to the setup used for metrics and traces using OTel, a setup of two gateway instances should be enough to handle the maximum number of logs allowed by a CLS (enterprise-plan) instance.

These results are based on a very basic logging pipeline and must be reevaluated as soon as the pipeline setup has been finalized.

Another important factor for the log gateway will be the resource limits configuration. For the tests executed here, no limits were applied.
