# Optimizing Memory Management in OpenTelemetry Applications

This document aims to guide developers in managing memory usage in OpenTelemetry applications. The primary challenge is mitigating the risk of out-of-memory (OOM) errors while optimizing application performance. The solution involves understanding memory management, garbage collection in Go runtime, and the impact of memory allocation strategies.

## Memory Allocation and OOM Errors

Memory can be allocated on the stack or heap. Stack allocations are short-lived and cheap, requiring no garbage collection (GC). Heap allocations are long-lived, more expensive, and require GC when no longer in use. Unintentional heap allocations can lead to OOM errors by putting pressure on the GC.

## Balancing Memory Usage and Garbage Collection

The goal is to balance memory usage and GC to prevent OOM errors while maintaining efficient application performance. This involves delaying GC cycles until necessary, ensuring the application never exceeds the memory threshold.

## Garbage Collection Targets in Go

Before Go 1.19, the GOGC environment variable was the only tool available to manage GC. It accepts a relative target compared to the current live heap size. However, this approach can be problematic for applications with large permanent live heaps.

## Introducing GOMEMLIMIT

Go 1.19 introduced GOMEMLIMIT, a feature that allows the specification of a soft memory cap. It complements GOGC, making the garbage collector more aggressive when necessary. It's a "soft" limit, meaning the Go runtime uses it as a target rather than a strict constraint.

## Evaluating GOMEMLIMIT with TracePipeline

We tested GOMEMLIMIT with TracePipeline, a memory-intensive application. Without GOMEMLIMIT, TracePipeline exceeded available memory, leading to OOM errors. With GOMEMLIMIT set, garbage collection behavior was more controlled, and memory usage remained within the specified limit.

![TracePipeline without GOMEMLIMIT](./assets/without-gomemlimit.jpg)

![TracePipeline with GOMEMLIMIT](./assets/with-gomemlimit.jpg)

## Conclusion

GOMEMLIMIT effectively mitigates OOM errors in heavily utilized Go applications. However, efficient memory allocation strategies remain crucial for optimal performance. While GOMEMLIMIT provides valuable guidance, careful consideration of application requirements and workload characteristics is imperative for robust memory management.

## Is GOMEMLIMIT Safe to Avoid OOM?

GOMEMLIMIT can help mitigate the risk of OOM errors, but it does not provide foolproof protection. Even with GOMEMLIMIT in place, a heavily utilized Go application must still prioritize efficient memory allocation strategies. Despite configuring an application with a GOMEMLIMIT, the application may encounter an OOM situation if circumstances necessitate additional memory allocation.

![TracePipeline with GOMEMLIMIT and OOM](./assets/with-gomemlimit-oom.jpg)

The OpenTelemetry project provides a comprehensive set of tools for monitoring and managing memory usage in OpenTelemetry applications. By leveraging OpenTelemetry's capabilities, such as memory limiter processor, enabling them to optimize application performance and reliability. As we experienced, the memory limiter processor can help mitigate the risk of encountering out-of-memory (OOM) errors, but it does not provide foolproof protection. Despite the benefits of memory limiter processor, it's important to note that memory management is a complex and multifaceted topic. To get our application to work efficiently, we need to understand the memory management and garbage collection behavior of the Go runtime. We also need to consider the impact of memory allocation strategies and the potential risks of OOM errors.

## Understanding Memory Management

There are two ways to allocate memory: on the stack or on the heap. A stack allocation is short-lived and typically very cheap. No Garbage Collection (GC) is required for stack allocation since the end of the function marks the end of the variable's lifetime. On the other hand, a heap allocation is long-lived and considerably more expensive. When allocating on the heap, the runtime must find a contiguous piece of memory where the new variable fits. Additionally, it must be garbage-collected when the variable is no longer used. Both operations are orders of magnitude more expensive than a stack allocation.

### Why Would You Run Out of Memory (OOM)?

Short-lived allocations end on the stack, and long-lived allocations end up on the heap. In reality, it's not always this simple. Sometimes you will end up with unintentional heap allocations. It's important to know because those allocations will put pressure on the GC, which is required for preventing unexpected OOM situations.

Long-lived memory is something you can estimate upfront or control at runtime. For example, if you have a full-blown cache application, you likely have some sort of limit. Either the cache would stop accepting new values when it's full or start dropping old cache entries. For instance, you could ensure that the cache never exceeds 2GB in size. Then you should be safe on your 4GB machine. The answer is "maybe", but "maybe" is not enough when the risk is running out of memory.

To understand why it is possible to encounter OOM in this situation, we need to look at when the garbage collector runs. We know that we have 2GB of live memory, and simply by using the application, we add a few short-lived heap allocations here and there. We don't expect them to stick around long-term, but since there is no GC cycle running at the moment, they will never be freed. Eventually, we will encounter OOM when intentionally and unintentionally live heap exceeds 4GB.

Now let's look at the other extreme: the Garbage Collector runs extremely frequently. Any time our heap reaches 2.1GB, it runs and removes the 100MB of temporary allocation. An OOM situation is improbable now, but we have far exceeded our cost target; the application might now spend 30-40%, maybe more, on GC. This is no longer efficient.

The optimal solution is the best of two worlds: to get as close to our limit as possible but never beyond it. This way we can delay GC cycles until they are necessary. This will make our application fast, but at the same time, we can be sure that it never crosses the threshold, which makes our application safe from being OOM-killed.

### Understanding Garbage Collection Targets in Go
We want to make sure we use memory we have without going above it. Before Go 1.19, you had only one knob to turn: the GOGC environment variable. This environment variable accepts a relative target compared to the current live heap size. The default value for GOGC is 100, meaning that the heap should double before GC should run again.

That works well for applications that have small permanent live heaps. For example, if your constant heap is just 50MB and you have 4GB available, you can double your heap targets any time before ever being in danger. If the application load increases and temporary allocation increases, the dynamic targets would be 100MB, 200MB, 400MB, 800MB, 1600MB, and 3200MB. The load must double seven times to cross the 4GB mark, making running out of memory extremely unlikely.

But now think back to our cache application example with a permanent 2GB live heap on a 4GB machine. Even the first doubling of the heap is highly problematic because the new target (4GB) would already reach the limit of the physical memory on the machine.

Before Go 1.19, there was not much we could do about this; GOGC was the only knob that we could turn. So we most likely picked a value such as GOGC=25. That means the heap could grow by 25% before GC kicks in. Our new target would now be 2.5GB; unless the load changes drastically, we should be safe from running OOM.

This will work only at a single snapshot in time and assume that we always start with a 2GB live heap. But what if fewer items are in the cache, and the live heaps are only 100 MB? That would make our heap's goal just 125MB. In other words, we would end up with constant GC cycles, and they would take up a lot of CPU time.

### Be Less Aggressive When Enough Memory Is Available, Be Very Aggressive When Less Memory Is Available

What we want to achieve is a situation where the GC is not very aggressive when a lot of memory is still available, but at the same time, the GC should become very aggressive when available free memory is tight. In the past, this was only possible with a workaround, the so-called "memory ballast" method. At the application startup, you would allocate a ballast, mostly a byte array that would take up a vast amount of memory, so you can make GOGC quite aggressive. Back to our example above, if you allocate a 2GB ballast and set GOGC=25, the GC will not run until 2.5GB memory is allocated.

## Introducing GOMEMLIMIT

With Go 1.19, the GOMEMLIMIT feature provides a better solution by allowing the specification of a soft memory cap. It complements the existing GOGC setting, making the garbage collector more aggressive when necessary.

### Understanding Soft Limits

GOMEMLIMIT is considered a "soft" limit, meaning that the Go runtime uses it as a target rather than a strict constraint. In situations where memory usage exceeds the limit, the runtime prefers fast failure to prevent resource contention and application stalls.

## Evaluating with TracePipeline
To illustrate the benefits of GOMEMLIMIT, we conducted tests with TracePipeline, a memory-intensive application. By comparing scenarios with and without GOMEMLIMIT, we observed differences in garbage collection behavior and memory usage.

### Results

- Without GOMEMLIMIT, TracePipeline exceeded available memory, leading to OOM errors.


![TracePipeline without GOMEMLIMIT](./assets/without-gomemlimit.jpg)

- With GOMEMLIMIT set, garbage collection behavior was more controlled, and memory usage remained within the specified limit.

![TracePipeline with GOMEMLIMIT](./assets/with-gomemlimit.jpg)


## Conclusion
Our experiments demonstrate the effectiveness of GOMEMLIMIT in mitigating OOM errors in heavily utilized Go applications. However, efficient memory allocation strategies remain crucial for optimal performance. While GOMEMLIMIT provides valuable guidance, careful consideration of application requirements and workload characteristics is imperative for robust memory management.

- With our experiments, we can prove that our TracePipeline could crash on a 2GiB Pod with a load test, even when a constant load is less than 2GiB.
- After using GOMEMLIMIT=1.8GiB, TracePipeline no longer crashed and could efficiently use the available memory.
- Before Go 1.19, the Go runtime could only set relative GC targets. That would make it very hard to use the available memory efficiently.

## Is GOMEMLIMIT Safe to Avoid OOM?

While setting a GOMEMLIMIT can help mitigate the risk of encountering out-of-memory (OOM) errors, it's important to note that it does not provide foolproof protection. Even with GOMEMLIMIT in place, a heavily utilized Go application must still prioritize efficient memory allocation strategies. As we've previously discussed, GOMEMLIMIT serves as a soft limit, meaning there's no absolute assurance that the application will consistently operate within its boundaries. The following memory snapshot exemplifies this scenario: despite configuring TracePipeline with a GOMEMLIMIT of 1.8GiB, the application experiences a significant increase in workload after surpassing this threshold. While the Go runtime endeavors to maintain compliance with the specified limits, if circumstances necessitate additional memory allocation, the application may ultimately encounter an OOM situation.

![TracePipeline with GOMEMLIMIT and OOM](./assets/with-gomemlimit-oom.jpg)