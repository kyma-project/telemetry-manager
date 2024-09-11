# 14. Telemetry Self-Monitoring Storage

Date: 2024-10-09

## Status

Proposed

## Context

The Telemetry module self-monitoring monitors the overall health of the system therefor availability and safe operation of self-monitoring is important. The self-monitoring data is used to detect issues in the Telemetry module and to provide insights into the system's health. The self-monitoring data is stored in a time-series database (TSDB) and is used to generate alerts. 
The current storage configuration and retention policy for the self-monitoring data are not well-defined. Currently, some installations face the issue that self-monitoring storage fills up and exceeds the storage limit despite the retention policies of 2 hours or 50 MBytes. 
The Telemetry self-monitoring data is stored in the Prometheus TSDB, which is designed for large-scale deployments. The amount of data collected by the Telemetry self-monitoring is actually small compared to the Prometheus capabilities (a few MBytes). Nevertheless, the storage size and retention policies must be carefully configured.


### Storage and Retention with TSDB

The TSDB storage size-based retention works as follows: It includes data blocks like the write-ahead-log (WAL), the checkpoints, the m-mapped chunks, and the persistent blocks. The TSDB counts all those data blocks to decide performing any retention.
Even if size of all those data blocks go beyond the configured retention size, only persistence blocks are deleted because the WAL, checkpoints, and m-mapped chunks that are required for normal operation of TSDB. The WAL segments can grow up to 128MB before compacting, and Prometheus will keep at least 3 WAL files; [so-called 2/3 rules](https://ganeshvernekar.com/blog/prometheus-tsdb-wal-and-checkpoint/#wal-truncation). To ensure that Telemetry self-monitoring doesn't exceed the storage limit, minimum storage volume size should be calculated to be at least 3 * WAL segment size + some more space for other data types.  

### TSDB Storage architecture and retention

For more information about the Prometheus storage architecture and retention policy, see [Prometheus TSDB: Compaction and Retention](https://ganeshvernekar.com/blog/prometheus-tsdb-compaction-and-retention).
For the TSDB WAL and checkpoint architecture, see [Prometheus TSDB: WAL and Checkpoint](https://ganeshvernekar.com/blog/prometheus-tsdb-wal-and-checkpoint/).


## Consequences

Even though the Telemetry self-monitoring collects very little data for operation (currently, a few MBytes), the storage size must be at least 500MByte for a normal and safe operation.