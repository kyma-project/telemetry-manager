# Metrics KPIs and Limit Test
# Setup
> **NOTE:** These are current test results where the max throughput was not reached. The results would be different when we have a script for setup with max throughput. The results should be refreshed then.



## Test Results

|                                       Test Description |     0.91      |        0.92 |
|-------------------------------------------------------:|:-------------:|------------:|
| Single Pipeline- Receiver Accepted Metric points / sec |      666      |         660 |
|  Single Pipeline-Exporter Exported Metric points / sec |      667      |         660 |
|                    Single Pipeline-Exporter Queue Size |       0       |           0 |
|              Single Pipeline-Pod Memory Usage (MBytes) | 91.8/90.98 MB | 91.8, 90.98 |
|                        Single Pipeline - Pod CPU Usage |   0.43/0.42   |  0.43, 0.42 |

