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

<div class="table-wrapper" markdown="block">

|       Version/Test | Single Pipeline (ci-traces) |                             |                     |                      |               | Multi Pipeline (ci-traces-m) |                             |                     |                      |               | Single Pipeline Backpressure (ci-traces-b) |                             |                     |                      |               | Multi Pipeline Backpressure (ci-traces-mb) |                             |                     |                      |               |
|-------------------:|:---------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|:----------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|:------------------------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|:------------------------------------------:|:---------------------------:|:-------------------:|:--------------------:|:-------------:|
|                    | Receiver Accepted Spans/sec | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Spans/sec  | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Spans/sec         | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Spans/sec         | Exporter Exported Spans/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|               0.91 |            19815            |            19815            |          0          |       137, 139       |     1, 1      |            13158             |            38929            |          0          |       117, 98        |   1.3, 1.3    |                    9574                    |            1280             |         509         |      1929, 1726      |   0.7, 0.7    |                    9663                    |            1331             |         510         |      2029, 1686      |   0.7, 0.7    |
|               0.92 |            21146            |            21146            |          0          |        72, 50        |     1, 1      |            12757             |            38212            |          0          |       90, 111        |   1.3, 1.1    |                    3293                    |            2918             |         204         |       866, 873       |   0.6, 0.6    |                    9694                    |            1399             |         510         |      1730, 1796      |   0.7, 0.7    |
|               0.93 |            19708            |            19708            |          0          |        69, 62        |     1, 1      |            12355             |            37068            |          0          |       158, 140       |   1.5, 1.2    |                    319                     |             324             |         237         |      874, 1106       |    0.1, 0     |                    8209                    |             865             |         510         |      1755, 1650      |   0.4, 0.4    |
|               0.94 |            19933            |            19934            |          0          |       110, 76        |     1, 1      |            13083             |            39248            |          0          |       94, 152        |   1.2, 1.4    |                    299                     |             299             |         214         |      1003, 808       |    0.1, 0     |                    8644                    |             916             |         169         |      1578, 1706      |   0.5, 0.5    |
|               0.95 |            20652            |            20652            |          0          |       133, 76        |    1, 0.8     |            13449             |            40350            |          0          |       150, 111       |   1.3, 1.4    |                    330                     |             328             |         239         |      931, 1112       |     0, 0      |                    8259                    |             929             |         170         |      1693, 1611      |   0.7, 0.6    |
|               0.96 |            20973            |            20807            |          0          |        66, 77        |     1, 1      |            13649             |            40403            |          0          |       133, 111       |   1.3, 1.5    |                    293                     |             295             |         233         |       946, 989       |    0, 0.1     |                    7683                    |             944             |         169         |      1558, 1593      |   0.4, 0.6    |
|               0.97 |            20543            |            20380            |          0          |       174, 92        |     1, 1      |            12807             |            37917            |          0          |       172, 107       |   1.4, 1.3    |                    315                     |             313             |         193         |      1001, 1028      |     0, 0      |                    8039                    |             953             |         168         |      1690, 1684      |   0.6, 0.4    |
| 0.97 w. GOMEMLIMIT |            19951            |            19795            |          0          |       76, 120        |    0.9, 1     |            13104             |            38794            |          0          |       340, 183       |   1.4, 1.4    |                   11670                    |             325             |         511         |      1869, 1754      |   0.4, 0.5    |                   20937                    |            1011             |         170         |      1694, 1712      |   0.9, 0.9    |
|             0.99.0 |            20724            |            20560            |          0          |        85, 81        |     1, 1      |            13319             |            39434            |          0          |       138, 137       |   1.2, 1.4    |                   11203                    |             298             |         508         |      1716, 1727      |   0.5, 0.5    |                   20666                    |             959             |         170         |      1721, 1695      |   0.9, 0.9    |
|            0.100.0 |            20134            |            19975            |          0          |       216, 71        |    0.9, 1     |            13665             |            40464            |          0          |       294, 296       |   1.3, 1.4    |                   11339                    |             314             |         511         |      1753, 1778      |   0.6, 0.5    |                   22654                    |             884             |         170         |      1671, 1674      |   0.9, 0.8    |
|            0.102.1 |            19914            |            19757            |          0          |        84, 78        |    1.1, 1     |            14407             |            42663            |          0          |       196, 117       |   1.4, 1.4    |                   11891                    |             306             |         511         |      1886, 1803      |   0.6, 0.4    |                   23236                    |             953             |         170         |      1663, 1688      |   0.8, 0.8    |
|      0.102.1 (new) |            21165            |            20999            |          0          |        75, 73        |     1, 1      |            13407             |            39703            |          0          |       147, 162       |   1.4, 1.4    |                   12040                    |             327             |         512         |      1718, 1701      |   0.5, 0.5    |                   22475                    |             904             |         170         |      1605, 1602      |   0.9, 0.9    |
|            0.103.0 |            20140            |            19982            |          0          |        65, 68        |     1, 1      |            12972             |            38400            |          0          |       146, 176       |   1.4, 1.4    |                   10663                    |             288             |         512         |      1707, 1707      |   0.5, 0.5    |                   19154                    |             969             |         170         |      1699, 1701      |     1, 1      |
|            0.104.0 |            19924            |            19766            |          0          |       94, 204        |   1.1, 0.9    |            12343             |            36536            |          0          |       136, 185       |   1.3, 1.4    |                   10761                    |             329             |         512         |      1741, 1738      |   0.5, 0.5    |                   17390                    |             927             |         170         |      1720, 1737      |    0.9, 1     |
|            0.105.0 |            19187            |            19084            |          0          |       268, 96        |     1, 1      |            12292             |            36383            |          0          |       144, 180       |   1.3, 1.4    |                   10846                    |             323             |         511         |      1717, 1699      |   0.5, 0.5    |                   19344                    |             940             |         510         |      1728, 1690      |     1, 1      |
|            0.106.1 |            20283            |            20123            |          0          |       96, 103        |     1, 1      |            12858             |            38067            |          0          |       136, 150       |   1.4, 1.4    |                   10727                    |             310             |         512         |      1719, 1731      |   0.5, 0.5    |                   19802                    |             920             |         510         |      1764, 1716      |     1, 1      |

</div>

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

|       Version/Test | Single Pipeline (ci-metrics) |                              |                     |                      |               | Multi Pipeline (ci-metrics-m) |                              |                     |                      |               | Single Pipeline Backpressure (ci-metrics-b) |                              |                     |                      |               | Multi Pipeline Backpressure (ci-metrics-mb) |                              |                     |                      |               |
|-------------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|                    | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec  | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|               0.92 |             5992             |             5993             |          0          |       225, 178       |   1.6, 1.5    |             4882              |            14647             |          0          |       165, 255       |   1.7, 1.8    |                     635                     |             636              |         114         |       770, 707       |     0, 0      |                     965                     |             1910             |         400         |      1694, 1500      |   0.1, 0.1    |
|               0.93 |             5592             |             5593             |          0          |       104, 100       |   1.6, 1.5    |             4721              |            14164             |          0          |       161, 175       |   1.8, 1.7    |                     723                     |             634              |         217         |       805, 889       |   1.4, 1.4    |                    1492                     |             1740             |         419         |      1705, 1535      |    0.2, 0     |
|               0.94 |             5836             |             5835             |          0          |       164, 244       |   1.6, 1.4    |             4873              |            14619             |          0          |       157, 228       |   1.8, 1.5    |                     870                     |             667              |         297         |       954, 782       |   0.3, 0.8    |                    1443                     |             1811             |         59          |      903, 1075       |    0, 0.1     |
|               0.95 |             6092             |             6091             |          0          |       96, 117        |   1.5, 1.5    |             5275              |            15827             |          0          |       185, 151       |   1.8, 1.7    |                     735                     |             634              |         243         |       824, 896       |     0, 0      |                    2325                     |             1809             |         170         |      1446, 1601      |   1.5, 1.6    |
|               0.96 |             4690             |             4689             |          0          |       171, 115       |   1.4, 1.4    |             4249              |            12748             |          0          |       156, 167       |   1.6, 1.6    |                     710                     |             577              |         226         |       717, 860       |   0.5, 1.1    |                    2638                     |             1738             |         165         |      1998, 1618      |   0.3, 0.3    |
|               0.97 |             4509             |             4510             |          0          |       107, 106       |   1.3, 1.4    |             4103              |            12308             |          0          |       171, 190       |   1.4, 1.6    |                     787                     |             681              |         261         |       710, 959       |   0.8, 1.2    |                    2710                     |             1847             |         170         |      1891, 1765      |   1.1, 1.2    |
| 0.97 w. GOMEMLIMIT |             4576             |             4576             |          0          |       107, 123       |   1.4, 1.4    |             3840              |            11522             |          0          |       148, 156       |   1.6, 1.5    |                     805                     |             585              |         347         |       781, 769       |   1.4, 1.4    |                    3690                     |             1828             |         170         |      1766, 1783      |   1.5, 1.6    |
|               0.99 |             4530             |             4531             |          0          |        97, 95        |   1.3, 1.4    |             4086              |            12259             |          0          |       179, 162       |   1.4, 1.6    |                     821                     |             609              |         388         |       756, 781       |    1.1, 1     |                    3604                     |             1743             |         170         |      1778, 1853      |   1.6, 1.5    |
|            0.100.0 |             4249             |             4249             |          0          |       120, 130       |   1.3, 1.4    |             3804              |            11413             |          0          |       193, 153       |   1.6, 1.3    |                     781                     |             590              |         367         |       743, 787       |   0.9, 0.5    |                    3370                     |             1924             |         170         |      1538, 1956      |   1.6, 1.6    |
|            0.102.1 |             4453             |             4454             |          0          |       100, 90        |   1.3, 1.3    |             3814              |            11445             |          0          |       187, 213       |   1.5, 1.4    |                     774                     |             553              |         375         |       783, 788       |    0, 0.1     |                    3333                     |             1805             |         170         |      1550, 1946      |   1.7, 1.7    |
|      0.102.1 (new) |             3868             |             3869             |          0          |       131, 107       |   1.2, 1.4    |             3958              |            11875             |          0          |       255, 178       |   1.5, 1.6    |                     840                     |             628              |         382         |       918, 888       |   0.5, 0.5    |                    3264                     |             1900             |         168         |      1843, 1648      |   1.6, 1.6    |
|            0.103.0 |             4665             |             4666             |          0          |       109, 132       |   1.4, 1.4    |             3913              |            11743             |          0          |       219, 156       |   1.6, 1.7    |                     798                     |             597              |         327         |       863, 843       |   0.4, 0.4    |                    3102                     |             1841             |         169         |      1826, 1799      |   1.6, 1.6    |
|            0.104.0 |             4906             |             4906             |          0          |       131, 134       |   1.4, 1.4    |             4177              |            12536             |          0          |       197, 234       |   1.7, 1.6    |                     800                     |             567              |         387         |       879, 829       |   0.5, 0.5    |                    3268                     |             1804             |         170         |      1848, 1802      |   1.6, 1.6    |
|            0.105.0 |             4546             |             4546             |          0          |       137, 142       |   1.5, 1.5    |             3165              |             9500             |          1          |       221, 224       |   1.7, 1.7    |                     807                     |             642              |         310         |       841, 825       |   0.5, 0.5    |                    2083                     |             1872             |         504         |      1755, 1747      |   1.4, 1.4    |
|            0.106.1 |             4698             |             4698             |          0          |       132, 128       |   1.5, 1.5    |             3583              |            10743             |          0          |       231, 253       |   1.7, 1.8    |                     787                     |             541              |         336         |       876, 846       |   0.5, 0.5    |                    1979                     |             1956             |         509         |      1591, 1637      |   1.3, 1.3    |

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

|       Version/Test | Single Pipeline (ci-metric-ag) |                              |                     |                      |               | Single Pipeline Backpressure (ci-metric-ag-b) |                              |                     |                      |               |
|-------------------:|:------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:---------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|                    |  Receiver Accepted Metric/sec  | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |         Receiver Accepted Metric/sec          | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
|               0.92 |             20123              |            20137             |          0          |       704, 747       |   0.2, 0.2    |                     19952                     |            15234             |          0          |       751, 736       |   0.3, 0.2    |
|               0.93 |             19949              |            19946             |          0          |       704, 729       |   0.2, 0.2    |                     16699                     |            16591             |         107         |       852, 771       |   0.2, 0.2    |
|               0.94 |             19957              |            19950             |          0          |       727, 736       |   0.2, 0.4    |                     19825                     |            19824             |          0          |      1046, 1090      |   0.2, 0.2    |
|               0.95 |             19648              |            19645             |          0          |       707, 734       |   0.3, 0.2    |                     19717                     |            19818             |          0          |       657, 996       |   0.2, 0.3    |
|               0.96 |             19937              |            19905             |         29          |       749, 699       |   0.2, 0.2    |                     19843                     |            19766             |         70          |       840, 995       |   0.2, 0.2    |
|               0.97 |             20120              |            20122             |          0          |       937, 996       |   0.2, 0.2    |                     19667                     |            19665             |          0          |       900, 961       |   0.3, 0.2    |
| 0.97 w. GOMEMLIMIT |             219981             |            19980             |          0          |       802, 689       |   0.2, 0.2    |                     19736                     |            19743             |          0          |       783, 862       |   0.2, 0.2    |
|               0.99 |             20139              |            20138             |          0          |       749, 792       |   0.2, 0.2    |                     20170                     |            20155             |          6          |       721, 730       |   0.2, 0.2    |
|            0.100.0 |             20067              |            20049             |          9          |       704, 700       |   0.2, 0.2    |                     20011                     |            20011             |          0          |       780, 704       |   0.2, 0.2    |
|            0.102.1 |             19883              |            19884             |          0          |       776, 733       |   0.2, 0.2    |                     20085                     |            20080             |          0          |       776, 718       |   0.2, 0.2    |
|      0.102.1 (new) |             20007              |            19989             |         15          |       697, 713       |   0.2, 0.2    |                     19967                     |            19964             |          0          |       731, 683       |   0.2, 0.2    |
|            0.103.0 |             19994              |            20038             |          0          |       684, 670       |   0.2, 0.2    |                     19989                     |            19998             |          0          |       724, 671       |   0.2, 0.2    |
|            0.104.0 |             19906              |            19898             |          6          |       689, 744       |   0.2, 0.2    |                     19818                     |            19823             |          0          |       685, 685       |   0.2, 0.2    |
|            0.105.0 |             20128              |            20126             |          0          |       767, 734       |   0.2, 0.2    |                     20084                     |            20093             |          0          |       692, 727       |   0.2, 0.2    |
|            0.106.1 |             20119              |            20101             |          7          |       824, 681       |   0.2, 0.2    |                     19658                     |            19655             |          0          |       725, 703       |   0.2, 0.2    |

</div>

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

#### Test Results

<div class="table-wrapper" markdown="block">

|        Version/Test |        Single Pipeline (ci-logs)        |                                          |                                 |                      |               |       Multi Pipeline (ci-logs-m)        |                                          |                                 |                      |               | Single Pipeline Backpressure (ci-logs-b) |                                          |                                 |                      |               | Multi Pipeline Backpressure (ci-logs-mb) |                                          |                                 |                      |               |
|--------------------:|:---------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:|:---------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:|:----------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:|:----------------------------------------:|:----------------------------------------:|:-------------------------------:|:--------------------:|:-------------:|
|                     | Input Bytes Processing Rate/sec (KByte) | Output Bytes Processing Rate/sec (KByte) | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage | Input Bytes Processing Rate/sec (KByte) | Output Bytes Processing Rate/sec (KByte) | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage | Input Bytes Processing Rate/sec (KByte)  | Output Bytes Processing Rate/sec (KByte) | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage | Input Bytes Processing Rate/sec (KByte)  | Output Bytes Processing Rate/sec (KByte) | Filesystem Buffer Usage (KByte) | Pod Memory Usage(MB) | Pod CPU Usage |
|               2.2.1 |                  5165                   |                   8541                   |              68518              |       172, 190       |     1, 1      |                  2009                   |                   2195                   |             102932              |       332, 320       |   0.9, 0.9    |                   5914                   |                   1498                   |              79247              |       184, 176       |    0.9, 1     |                   1979                   |                   489                    |              83442              |       310, 322       |   0.9, 0.9    |
|               2.2.2 |                  5159                   |                   7811                   |              75545              |       171, 170       |     1, 1      |                  1910                   |                   2516                   |             103780              |       324, 324       |   0.9, 0.9    |                   5857                   |                   1513                   |              72494              |       189, 200       |     1, 1      |                   1860                   |                   421                    |              90852              |       314, 322       |   0.9, 0.9    |
|   2.2.2 (new setup) |                  5445                   |                   9668                   |              68453              |       248, 981       |     1, 1      |                  6201                   |                   2747                   |              89291              |       544, 720       |     1, 1      |                   6009                   |                   1723                   |              58982              |       650, 682       |     1, 1      |                   6111                   |                   385                    |             108909              |       686, 931       |   0.9, 0.9    |
|               3.0.3 |                  9483                   |                  22042                   |              53251              |       366, 681       |     1, 1      |                  10737                  |                   8785                   |             115232              |       953, 568       |   0.9, 0.9    |                  10425                   |                   4610                   |              80614              |       856, 704       |   0.9, 0.9    |                  10955                   |                   1724                   |              87530              |       503, 594       |   0.9 ,0.9    |
|               3.0.4 |                  4341                   |                   8296                   |              35628              |       971, 726       |    0.1,0.1    |                  1201                   |                   544                    |             103624              |       652, 815       |     0, 0      |                   932                    |                   297                    |              37663              |       615, 726       |    0.1,0.1    |                   1477                   |                   171                    |             108885              |       530, 566       |    0, 0.1     |
| 3.0.7 (old metrics) |                  4241                   |                   7782                   |              47586              |       815,1021       |    0.7,0.1    |                  3809                   |                   1968                   |             107529              |       837,965        |     0.4,0     |                   3472                   |                   1093                   |              33818              |       792,597        |     0,0.1     |                   2180                   |                   177                    |              87052              |       708,631        |     0,0.1     |
| 3.0.7 (new metrics) |                  4036                   |                   7173                   |              31689              |       825,852        |    0.1,0.1    |                  2481                   |                   1852                   |             104689              |       747,395        |     0.1,0     |                   1520                   |                   484                    |              37907              |       561,731        |    0.1,0.1    |                   807                    |                    58                    |              94365              |       544,211        |      0,0      |
|         3.0.7 (new) |                  9514                   |                  30273                   |              30263              |       105, 113       |     1, 1      |                  9027                   |                  23850                   |             1521511             |       186, 552       |    1, 0.7     |                   7285                   |                   8357                   |             1891569             |       662, 668       |   0.8, 0.8    |                   5602                   |                   2619                   |             5249308             |       680, 713       |   0.5, 0.5    |
|               3.1.3 |                  8922                   |                  28278                   |              34609              |       105,107        |    0.8,0.9    |                  4542                   |                   9605                   |             2676743             |       601,528        |    0.4,0.4    |                   3764                   |                   4216                   |             1896390             |       620,636        |    0.4,0.4    |                   3336                   |                   1499                   |             4892724             |       678,698        |    0.3,0.3    |
</div>

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

#### Test Results

The main KPIs to track performance changes are **scrape samples per sec** and **total series created**. These values
should be in the range of 15-22 samples/sec and 200-350 series, respectively.
Other metrics to track are **CPU** and **memory usage** of the self-monitor Pods. Both are directly influenced by the
number of series created and the scrape samples/sec: more samples and series created increase the memory and CPU usage
of the self-monitor Pods.

<div class="table-wrapper" markdown="block">

| Version/Test | Default (ci-self-monitor) |                      |                        |                                  |                      |               |
|-------------:|:-------------------------:|:--------------------:|:----------------------:|:--------------------------------:|:--------------------:|:-------------:|
|              |    Scrape Samples/sec     | Total Series Created | WAL Storage Size/bytes | Head Chunk Storage Size in bytes | Pod Memory Usage(MB) | Pod CPU Usage |
|       2.45.5 |           15.4            |         157          |           -            |              131072              |          62          |       0       |
|  2.45.5(new) |           15.4            |         325          |         127633         |                0                 |          43          |       0       |
|       2.53.0 |           20.4            |         210          |           -            |                0                 |          36          |       0       |
|       2.53.1 |           20              |         333          |         135557         |                0                 |          37          |       0       |
</div>
