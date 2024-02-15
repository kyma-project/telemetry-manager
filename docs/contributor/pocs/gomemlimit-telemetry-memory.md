# GOMEMLIMIT for Telemetry Components

The Go version 1.19 brings new `GOMEMLIMIT` feature can help us both increase GC-related performance as well as avoid GC-related out-of-memory (OOM) situation.

## Why would you run OOM

There are two ways to allocate memory: On stack or the heap. A stack allocation is short-lived and typically very cheap. No Garbage Collection is required for stack allocation as the end of the function is also the end of the variable's lifetime. 
On the other hand, a heap allocation is long-lived and considerably more expensive. When allocating on the heap, the runtime must find a contiguous piece of memory where the new variable fits.
Additionally, it must be garbage-collected when the variable is no longer used. Both operations are orders of magnitudes more expensive than a stack allocation.

Short-lived allocations which end on the stack and long-lived allocations which end up on the heap. In reality, it's not always this simple. Sometimes you will end up with unintentionally heap allocation.
That is important to know because those allocations will put pressure on the GC, which is required for an unexpected OOM situation.

Log lived memory is something you can estimate upfront or control at runtime. For example, if you have a full-blown cache application, you have most likely some sort of limit. Either the cache would stop accepting new values when it's full or start dropping old cache entries.
For example, you could make sure that the cache never exceeds 2GB in size. Then you should be safe on your 4GB machine. The answer is "maybe", but maybe is not enough when the risk is of running out of memory.

To understand why it is possible to run OOM in this situation, we need to look at when the garbage collector runs. We know that we have 2GB of live memory, and simply from using application we add a few short-lived heap allocations here and there.
We don't expect them stick around long-term, but since there is no GC cycle running at the moment, they will never be freed. Eventually, we will run OOM when intentionaly and unintentionally live heap exceeds 4GB.

Now let's look at the other extreme, Garbage Collector runs extremely frequently. Any time our heap reaches 2.1GB, it runs and removes the 100MB of temporary allocation. 
An OOM situation is un-probable now, but we have far exceeded our cost target, application might now spend 30-40%, maybe more, on GC. This is no longer efficient.

Optimal solution is best of two worlds, to get as close to our limit as possible but never beyond it. This way we can delay GC cycles until they are necessary.
This will make our application fast but at the same time we can be sure that never cross the threshold, that makes our application safe from being OOM-killed

### Go GC targets

We want to make sure use of memory we have without going above it, before Go 1.19 you had only one knob to turn, the `GOGC` environment variable. This environment variable accept a relative target com,pared the current live heap size.
The default value for GOGC is 100 and meaning that the heap should double before GC should run again.

That works well for application that have small permanent live heaps, for example if your constant heap is just 50MB and you have 4GB available, you can double your heap targets any times before ever being in danger. 
If application load increases and temporary allocation increase, the dynamic targets would be 100MB, 200MB, 400MB, 800MB, 1600MB, and 3200MB. The load must double seven times to cross the 4GB mark, here running out of memory is extremely unlikely.

But now think back to our cache application example with a permanent 2GB live heap on 4GB machine. Event the forts doubling of the heap is highly problematic because the new target (4GB) would already reach the limit of the physical memory on the machine.

Before GO 1.19 there was not to much we could do about this, GOGC was the only knob that we could turn. So we most likely picked a value such `GOGC=25. That means the heap could grow by 25% for GC kick-in. Our new target would be now 2.5GB, unless the load change drastically we should be safe from running OOM.

This will work only at a single snapshot in time and pretended that we always start with 2GB live heap. But what if fewer items are in the cache and the live heaps is only 100MB. that would make our heaps goal just 125MB, in other words we would end-up with constant GC cycles and they would take up a lot of CPU time.


### Be less aggressive when enough memory available, be very aggressive when less memory available

What we want reach is, a stuation where the GC is not very aggressive when a lot of memory is still available, at the same time the GC should become very aggressive when available free memory is tight.
In the passt this was only possible with a workaround, the so called `memory ballast` method. At hte application startup, wou would allocate a ballast, mostly a byte array that would take up a vast amount of memory, so you can make GOGC quite aggressive.
Back to our example above, if you allocate a 2GB ballast and set `GOGC=25`, the GC will not run until 2.5GB memory is allocated. 

## GOMEMLIMIT

While of using virtual memory as ballast improve situation, it is still a workaround. With Go 1.19 we finally got a better solution, the GOMEMLIMIT allows specify a soft memory cap.
It does not replace GOGC but works in combination. We can set GOGC with a scenario in which memory always available and the same time we can trust that GOMEMLIMIT automatically makes the GC more aggressive when necessary.

When the live heap is log e.g. 100MB we can delay the next GC cylce until the heap has doubled, but when the heas has grown close to the limit, the GC runs more often to prevent us from running OOM. 

### Soft limit

The GO docs explicitly write GOMEMLIMIT a `soft` limit, that means the GO runtime does not guarantee that memory usage will exceed the limit, instead it uses as a target.
The goal s to fail fast in an impossible to solve situation, let assume we set te limit to a value just a few kilobytees larger than the live heap, the GC will have to run constantly.
We would be in a situation where the regular and GC execution would compete for the same resources, the application would stall and since there is no way out other than running with more memory the GO runtime prefers an OOM situation.

All the usable memory has been used up, and nothing can be freed anymore. That is a failure scenario, and fast failure is preferred. That makes the limit a `soft` limit.

## Test with TracePipeline

The test goal is care about OOM safety as well as about throughput performance of TracePipeline, the TracePipeline is a memory intensive application and is a perfect candidate to benefit from GOMEMLIMIT.

For this experiment, we wil use TracePipeline with OpenTelemetry Collector version 0.92.
We will load TracePipeline with huge amount of traces and observe GC behaviour and memory usage during test and see when run OOM, in all scenarios same test data used.

### Without GOMEMLIMT

For first run, we don't use GOMEMLIMIT, GOGC is set to 100, and available memory is 2Gib

![TracePipeline without GOMEMLIMIT](./assets/without-gomemlimit.jpg)

As we can see from memory snapshot above, application start around ~980Mib live heap and next GC target is ~1.88Gib. After GC cycle from 1.88Gib is new target would be ~3.7Gib which already above available memory and application will run OOM.

### With GOMEMLIMIT

For second run, we use GOMEMLIMIT with the value 1.8Gib, and GOGC is set to 100, and available memory is 2Gib.

![TracePipeline with GOMEMLIMIT](./assets/with-gomemlimit.jpg)

As we can see situation changed dramatically, no GC cycles until we reach our soft limit 1.8Gib also GC less aggressive but after we get closer to our soft limit 1.8Gib the Gc get more aggressive and run often to recover memory.

In summary GOMEMLIMIT made the GC more aggressive when less memory available, The memory usage not exceeded our soft limit 1.8Gib

## Conclusion

- With our experiments we are able to prove we could get our TracePipeline crashed on a 2Gib Pod with load test, even when constant load is less than 2Gib 
- After using `GOMEMLIMIT=1.8Gib` TracePipeline no longer crashed and could efficiently use the available memory
- Before Go 1.19, the Go runtime could only set relative GC targets. That would make it very hard to use the available memory efficiently

Does that mean that GOMEMLIMIT is safe to avoid OOM? No,  a Go application that gets heavy isage still has to ensure allocation efficiency, simply setting a GOMEMLIMIT will not guarantee OOM will not happen.
As we explain above te GOMEMLIMIT is a soft limit and there are no guarantee application will stay with in the limit, the memory snapshot below show exactly this situation.
The TracePipeline configured with GOMEMLIMIT 1.8Gib but application get much more load after reach the configured limit. In this situation Go runtime will try to keep application within the limits for a while but when there are no other than allocate more memory, application will run OOM.

![TracePipeline with GOMEMLIMIT and OOM](./assets/with-gomemlimit-oom.jpg)
