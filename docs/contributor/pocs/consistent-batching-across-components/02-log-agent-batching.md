# Investigate Exporter Batcher Configuration with Synchronous Log Emitter in Stanza

This investigation determines the ideal exporter batcher configuration after enabling the `SynchronousLogEmitter`. The [stanza-filelog-batching](01-stanza-filelog-batching.md) investigation recommends this change.

The chosen configuration must satisfy the following criteria:

- **Batch size** must be large enough to avoid sending too many small requests to the backend, but small enough to stay within the backend's maximum payload size.
- **Throughput** must remain the same before and after enabling the `SynchronousLogEmitter`.
- **Resource consumption** must remain the same before and after enabling the `SynchronousLogEmitter`.
- **Backpressure** must propagate back to the receiver.

## Understand the Exporter Batcher Architecture

The exporter batcher uses a pull-based batching model with the following data path:

1. **Queue** - The `Send()` method places individual requests into the queue using `queue.Offer()`.
2. **Consume** - The queue's consumers call the batcher's `Consume()` method to pull requests and accumulate them into batches.
3. **Flush** - The batcher maintains a `currentBatch` and flushes it when the batch reaches the minimum size or the flush timeout expires.

Requests entering the queue come directly from the previous component in the OTel pipeline. The batcher then re-batches these requests into larger batches on the consumer side.

### Configure the Exporter Batcher

The exporter batcher has several configuration parameters, all documented in the [exporterhelper package](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper).

The `sending_queue` and the `batch` each have an independent `sizer` setting that determines the unit of measurement:

- The `sending_queue` defaults to the `requests` sizer. This means `queue_size` counts the number of individual requests. Each request can contain many log records.
- The `batch` defaults to the `items` sizer. This means `min_size` and `max_size` count individual log records.
- The `bytes` sizer is also available. It measures the serialized byte size of each request. This gives precise control over memory usage and batch payload sizes but requires serializing every request to calculate its size.

When both sizers are set to the same unit, the `queue_size` must not be smaller than the batch `min_size`.

Even though the `sending_queue` and `batch` can use different sizers, `requests` and `items` respectively, the `queue_size` must still be greater than or equal to the batch `min_size` in practice. The following scenario illustrates why.

The `SynchronousLogEmitter` can produce very small batches, as small as a single log record per request, because each reader emits independently. Consider a worst-case scenario where 1024 Pods each produce 1 log per second. Each log becomes a separate single-record request in the queue.

If `queue_size` is smaller than `min_size`, for example `queue_size: 1000` with `min_size: 1024`, the batcher stalls:

1. The queue fills up to its capacity of `1000` requests, holding 1000 log records with one record per request.
2. The batcher needs `1024` records to reach `min_size` and flush, but the queue can hold only `1000`.
3. The `flush_timeout` has not elapsed yet, so a time-based flush does not trigger either.
4. New requests cannot enter the queue because it is full, and the batcher cannot drain the queue because the batch is not large enough to flush.

The queue stays fully occupied until the `flush_timeout` expires. At that point, the batcher flushes whatever it has accumulated. During this window, all incoming requests block. Setting `queue_size` to at least `min_size`, in this case `1024`, prevents this bottleneck.

## Identify Backend Requirements

The backend services impose payload size and rate limits that constrain the batch configuration.

| Backend           | Maximum Payload Size                                            | Rate Limit                                                                                |
|-------------------|-----------------------------------------------------------------|-------------------------------------------------------------------------------------------|
| SAP Cloud Logging | `4 MiB` per request. Requests exceeding this limit return `413` | 100 x 2 KiB logs/s for Standard plan, 1000 x 2 KiB logs/s for Large plan                 |
| SAP Cloud ALM     | `1 MB` for JSON payloads, `150 KB` for protobuf binary payloads | `100` requests/minute for production plans, `100` requests/24 hours for development plans |

These limits mean that the exporter batcher must produce batches small enough to stay within the maximum payload size of each backend.

## Calculate Batch Sizes

This section establishes the memory baseline for the existing log agent and defines the constraint that any new configuration must preserve.

### Establish the Baseline: Existing Log Agent Memory Footprint

The existing log agent uses the `BatchingLogEmitter`, which coalesces log records from all readers into batches of up to `100` records. The exporter queue uses the default configuration with a `requests` sizer and a `queue_size` of `1000`.

Each queue slot holds one request, and each request contains up to `100` log records. With an average log record size of ~2 KB, the theoretical maximum memory the queue consumes is:

```
2 KB/record x 100 records/request x 1000 requests = 200 MB
```

### Preserve the Memory Footprint After Enabling the SynchronousLogEmitter

The `SynchronousLogEmitter` also produces batches of up to `100` records, but it batches per reader instead of across all readers. This means the exporter queue receives more, potentially smaller, requests. The exporter batcher must be configured so that the log agent stays within the same ~200 MB memory envelope while meeting the criteria listed at the top of this document.

## Evaluate Configuration Options

This section tests three exporter batcher configurations and compares their behavior against the baseline. All tests use 100 logs/s per Pod with 10 Pods over 300s. The stanza pipeline inserts the original log into the `log.original` field, duplicating the content into the log body. As a result, original 2 KB logs become ~4 KB after the pipeline.

The following table summarizes the results across all configurations:

| Category    | Metric                          | Baseline                                                  | Items 1024/1024                                              | Items 1024/512                                                        | Bytes 200MB/2-4MB                                                 |
|-------------|---------------------------------|-----------------------------------------------------------|--------------------------------------------------------------|-----------------------------------------------------------------------|-------------------------------------------------------------------|
| Description |                                 | Batching emitter only, no exporter batcher                | Batching emitter + queue=1024, batch=1024                    | Batching emitter + queue=1024, batch=512                              | Batching emitter + queue=200MB, batch=2-4MB                       |
| Input       | Queue capacity                  | 1000 requests                                             | 1024 requests                                                | 1024 requests                                                         | 200 MB                                                            |
|             | Batch min size                  | N/A                                                       | 1024 records                                                 | 512 records                                                           | 2 MB                                                              |
|             | Batch max size                  | N/A                                                       | 1024 records                                                 | 512 records                                                           | 4 MB                                                              |
| Output      | Records accepted                | 75602                                                     | 75602                                                        | 75602                                                                 | 75602                                                             |
|             | Records sent successfully       | 75602                                                     | 850                                                          | 75602                                                                 | 75602                                                             |
|             | Records lost                    | 0                                                         | 74752                                                        | 0                                                                     | 0                                                                 |
|             | gRPC calls (OK)                 | 1235                                                      | 1                                                            | 148                                                                   | 156                                                               |
|             | gRPC calls (RESOURCE_EXHAUSTED) | 0                                                         | 73                                                           | 0                                                                     | 0                                                                 |
|             | Avg records/gRPC call           | ~61                                                       | ~850                                                         | ~511                                                                  | ~485                                                              |
|             | Avg bytes/gRPC call             | ~273 KB                                                   | N/A                                                          | ~2.28 MB                                                              | ~2.17 MB                                                          |
|             | CPU total                       | 41.16s                                                    | 38.56s                                                       | 39.34s                                                                | 39.09s                                                            |
|             | CPU rate                        | ~116 ms/s                                                 | ~109 ms/s                                                    | ~110 ms/s                                                             | ~109 ms/s                                                         |
|             | RSS                             | 240 MB                                                    | 334 MB                                                       | 252 MB                                                                | 256 MB                                                            |
|             | `heap_alloc`                    | 59.3 MB                                                   | 137.9 MB                                                     | 81.9 MB                                                               | 71.5 MB                                                           |
|             | `total_alloc`                   | 2.490 GB                                                  | 3.137 GB                                                     | 2.760 GB                                                              | 2.790 GB                                                          |
|             | `total_alloc` rate              | ~7.0 MB/s                                                 | ~8.9 MB/s                                                    | ~7.7 MB/s                                                             | ~7.8 MB/s                                                         |
| Conclusion  |                                 | Stable, 1 gRPC call per emitter flush, no data loss       | Batch size ~4 MB exceeded backend limit. 74752 records lost. | ~9x fewer gRPC calls, no data loss, slightly higher RSS than baseline | ~9x fewer gRPC calls, predictable memory bound, no data loss      |

The following subsections analyze each configuration in detail.

### Items Sizer with Batch Size 1024

The following configuration uses the `items` sizer and aligns batch sizes with other components:

| Parameter                             | Value                | Rationale                                                                          |
|---------------------------------------|----------------------|------------------------------------------------------------------------------------|
| `sending_queue::sizer`                | `requests` (default) | Each queue slot holds one request, regardless of how many records it contains.     |
| `sending_queue::queue_size`           | `1024`               | Provides sufficient buffer for incoming batches.                                   |
| `sending_queue::batch::sizer`         | `items` (default)    | The batcher counts individual log records when deciding whether to flush.          |
| `sending_queue::batch::min_size`      | `1024`               | Matches the batch size used by other components.                                   |
| `sending_queue::batch::max_size`      | `1024`               | Matches the batch size used by other components.                                   |
| `sending_queue::batch::flush_timeout` | `10s`                | Matches the flush interval used by other components.                               |

The same assumptions as the baseline apply, that is, 2 KB per record and up to 100 records per request from the `SynchronousLogEmitter`. The maximum queue memory is:

```
2 KB/record x 100 records/request x 1024 requests ≈ 205 MB
```

This is within an acceptable range of the original 200 MB baseline.

**Problem: batch payload exceeds backend limits.** With a `max_size` of `1024` records, each batch reaches approximately:

```
~4 KB/record x 1024 records ≈ 4 MB
```

This exceeds the SAP Cloud Logging maximum payload size of `4 MiB` and the default gRPC maximum request size. As the comparison table shows, 73 gRPC calls return `RESOURCE_EXHAUSTED` and 74752 out of 75602 records are lost. RSS and `heap_alloc` are also significantly higher than the baseline because the batcher accumulates large batches that it cannot deliver.

### Items Sizer with Batch Size 512

Reducing `min_size` and `max_size` to `512` keeps batches within backend payload limits. As the comparison table shows, gRPC calls drop from 1235 to 148, approximately a 9x reduction, with the exporter batcher assembling ~511 records per call compared to ~61 in the baseline. CPU and memory are essentially equivalent to the baseline. RSS is slightly higher, approximately 12 MB more, likely because the batcher holds a larger in-flight buffer. The difference is small and within normal variation.

This configuration solves the payload size issue, but it does not directly control memory usage. The `items` sizer counts records without knowing their byte size, so memory consumption depends on the actual size of the logs passing through the pipeline.

### Bytes Sizer

The `bytes` sizer measures the serialized byte size of each request, giving direct control over both queue memory and batch payload sizes. The OpenTelemetry documentation notes that the `bytes` sizer is the least performant of the three sizers because it must serialize every request to calculate its size. The `items` sizer only counts records, and the `requests` sizer does not need to perform any calculation.

Testing both sizers under the same conditions shows no meaningful difference in CPU or memory usage. Efforts to reproduce a scenario where the `bytes` sizer causes measurable performance degradation are unsuccessful.

The following configuration uses the `bytes` sizer to bound queue memory and respect backend payload limits:

| Parameter                             | Value    | Rationale                                                                                                                |
|---------------------------------------|----------|--------------------------------------------------------------------------------------------------------------------------|
| `sending_queue::sizer`                | `bytes`  | Bounds the queue memory usage to a fixed byte limit.                                                                     |
| `sending_queue::queue_size`           | `200 MB` | Preserves the ~200 MB memory envelope of the existing log agent.                                                         |
| `sending_queue::batch::sizer`         | `bytes`  | The batcher measures the serialized byte size of log records when deciding whether to flush.                              |
| `sending_queue::batch::min_size`      | `2 MB`   | With an average log size of ~2 KB, each batch holds approximately 1000 log records.                                      |
| `sending_queue::batch::max_size`      | `4 MB`   | Matches the maximum payload size limit for SAP Cloud Logging and gRPC requests. Records exceeding this limit are split.  |
| `sending_queue::batch::flush_timeout` | `10s`    | Matches the flush interval used by other components.                                                                     |

As the comparison table shows, gRPC calls drop from 1235 to 156, approximately 9x, with the bytes batcher assembling ~2.17 MB per call compared to ~273 KB in the baseline. CPU is slightly lower with the exporter batcher. RSS and `heap_alloc` are slightly higher, which is expected because the batcher holds a larger in-flight buffer to accumulate to its byte threshold before flushing. No records are lost.

Other components currently use `batchprocessor`, which does not support sizers other than `items`. Using sizers other than `items` would require migrating all other components to the exporter batcher. The [batch processor migration ADR](../../arch/029-batch-processor-migration-to-exporterhelper.md) investigates this migration and identifies certain risks and unwanted behavior.

## Conclusions

The batching configuration cannot be consistent across all components because other components use `batchprocessor` with the `items` sizer. The log agent benefits from using the `bytes` sizer to bound queue memory usage and control batch sizes based on payload size. Tests show little to no performance degradation from introducing the exporter batcher.

The recommended configuration uses the `bytes` sizer to ensure that batches exceeding `4 MB` are split and batches of approximately `2 MB` are flushed:

```yaml
sending_queue:
  queue_size: 200000000 # 200 MB
  sizer: bytes
  batch:
    min_size: 2000000 # 2 MB
    max_size: 4000000 # 4 MB
    flush_timeout: 10s
```
