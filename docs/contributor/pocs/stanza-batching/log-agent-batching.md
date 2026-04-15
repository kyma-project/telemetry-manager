# Investigate Exporter Batcher Configuration with Synchronous Log Emitter in Stanza

This investigation determines the ideal exporter batcher configuration after enabling the `SynchronousLogEmitter`. The [stanza-filelog-batching](stanza-filelog-batching.md) investigation recommends this change.

The chosen configuration must satisfy the following criteria:

- **Batch size** must match other components, that is, `1024` records with a `10s` flush timeout.
- **Throughput** must remain the same before and after enabling the `SynchronousLogEmitter`.
- **Resource consumption** must remain the same before and after enabling the `SynchronousLogEmitter`.
- **Backpressure** must propagate back to the receiver.

## Architecture Overview

The exporter batcher uses a pull-based batching model with the following data path:

1. Individual requests are queued first.
2. Batcher consumes from queue. The queue's consumers pull requests and accumulate them into batches.
3. Batches are created during consumption. The batcher maintains a currentBatch and flushes it when it reaches minimum size or timeout.

Requests entering the queue come directly from the previous component in the OTel pipeline. The batcher then re-batches these requests into larger batches on the consumer side.

### Configure the Exporter Batcher

The exporter batcher has several configuration parameters, all documented in the [exporterhelper package](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper).

The `sending_queue` and the `batch` each have an independent `sizer` setting that determines the unit of measurement:

- The `sending_queue` defaults to the `requests` sizer. This means `queue_size` counts the number of individual requests. Each request can contain many log records.
- The `batch` defaults to the `items` sizer. This means `min_size` and `max_size` count individual log records.

When both sizers are set to the same unit, the `queue_size` must not be smaller than the batch `min_size`. Even when the sizers differ, this constraint matters in practice. See [Set `queue_size` to at Least Batch `min_size`](#set-queue_size-to-at-least-batch-min_size) for a detailed explanation.

## Calculate Batch Sizes

This section derives the exporter batcher configuration values from the existing log agent memory footprint.

### Establish the Baseline: Existing Log Agent Memory Footprint

The existing log agent uses the `BatchingLogEmitter`, which coalesces log records from all readers into batches of up to `100` records. The exporter queue uses the default configuration with a `requests` sizer and a `queue_size` of `1000`.

Each queue slot holds one request, and each request contains up to `100` log records. With an average log record size of ~2 KB, the theoretical maximum memory the queue consumes is:

```
2 KB/record x 100 records/request x 1,000 requests = 200 MB
```

### Preserve the Memory Footprint After Enabling the SynchronousLogEmitter

The `SynchronousLogEmitter` also produces batches of up to `100` records, but it batches per reader instead of across all readers. This means the exporter queue receives more, potentially smaller, requests. The exporter batcher must be configured so that the log agent stays within the same ~200 MB memory envelope while meeting the criteria listed at the top of this document.

### Choose the Configuration

The following configuration keeps the maximum queue capacity close to 200 MB and aligns batch sizes with other components:

| Parameter                             | Value                | Rationale                                                                                                                                                                          |
|---------------------------------------|----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `sending_queue::sizer`                | `requests` (default) | Each queue slot holds one request, regardless of how many records it contains.                                                                                                     |
| `sending_queue::queue_size`           | `1024`               | Provides sufficient buffer for incoming batches. See the [Set `queue_size` to at Least Batch `min_size`](#set-queue_size-to-at-least-batch-min_size) section for the rationale behind this value. |
| `sending_queue::batch::sizer`         | `items` (default)    | The batcher counts individual log records when deciding whether to flush.                                                                                                          |
| `sending_queue::batch::min_size`      | `1024`               | Matches the batch size used by other components.                                                                                                                                   |
| `sending_queue::batch::max_size`      | `1024`               | Matches the batch size used by other components.                                                                                                                                   |
| `sending_queue::batch::flush_timeout` | `10s`                | Matches the flush interval used by other components.                                                                                                                               |

The same assumptions apply, that is, 2 KB per record and up to 100 records per request from the `SynchronousLogEmitter`. The maximum queue memory is:

```
2 KB/record x 100 records/request x 1,024 requests ≈ 205 MB
```

This is within an acceptable range of the original 200 MB baseline.

### Set `queue_size` to at Least Batch `min_size`

Even though the `sending_queue` and `batch` use different sizers, `requests` and `items` respectively, the `queue_size` must still be greater than or equal to the batch `min_size` in practice. The following scenario illustrates why.

The `SynchronousLogEmitter` can produce very small batches, as small as a single log record per request, because each reader emits independently. Consider a worst-case scenario where 1,024 Pods each produce 1 log per second. Each log becomes a separate single-record request in the queue.

If `queue_size` is smaller than `min_size`, for example `queue_size: 1000` with `min_size: 1024`, the batcher stalls:

1. The queue fills up to its capacity of `1000` requests, holding 1,000 log records with one record per request.
2. The batcher needs `1024` records to reach `min_size` and flush, but the queue can hold only `1000`.
3. The `flush_timeout` has not elapsed yet, so a time-based flush does not trigger either.
4. New requests cannot enter the queue because it is full, and the batcher cannot drain the queue because the batch is not large enough to flush.

The queue stays fully occupied until the `flush_timeout` expires. At that point, the batcher flushes whatever it has accumulated. During this window, all incoming requests block. Setting `queue_size` to at least `min_size`, in this case `1024`, prevents this bottleneck.
