# 14. Telemetry Self Monitoring Storage

Date: 2024-10-09

## Status

Proposed

## Context

The Telemetry module self-monitoring is crucial for the overall health of the system. The self-monitoring data is used to detect issues in the Telemetry module and to provide insights into the system's health. The self-monitoring data is stored in a time-series database (TSDB) and is used to generate alerts. 
The current storage architecture and retention policy for the self-monitoring data are not well defined, currently, some installation faces the issue self-monitoring storage fill-up and exceed the storage limit despite the retention policies 2 hours or 50 MBytes. 
The Telemetry self-monitoring data is stored in the Prometheus TSDB, which is designed for large scale deployments, the amount data collected by the Telemetry self-monitoring is actually small compared to the Prometheus capabilities (currently few MBytes) nevertheless the storage size and retention policies have to be carefully configured.


### Storage and Retention with TSDB

The TSDB storage size-based retention works in a way, it includes data blocks the WAL, checkpoints, m-mapped chunks, and persistent blocks. The TSDB although counts all of those storage blocks to decide any deletion, the WAL, checkpoints, and m-mapped chunks required for normal operation of TSDB.
Only persistence blocks are deleted even if all those data blocks go beyond the configured retention size. The WAL segments can grow up to 128MB before compacting, and Prometheus will keep at least 3 WAL files so called 2/3 rules. To ensure the Telemetry self-monitoring doesn't exceed the storage limit, minimum storage volume size should be calculated at least 3 * WAL segment size + some more space for other data types.  

### TSDB Storage architecture and retention

For more information and insight into the Prometheus storage architecture and retention policy, please refer to the [TSDB blog series](https://ganeshvernekar.com/blog/prometheus-tsdb-compaction-and-retention).

## Consequences

The Telemetry self-monitoring requires or collect very small amount of data for operation (currently few MBytes), despite the small amount of data, the storage size have to be at least 500MByte for a normal and safe operation.


