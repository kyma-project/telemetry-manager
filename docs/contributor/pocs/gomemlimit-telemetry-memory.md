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

### GC targets are always relative

We want to make sure use of memory we have without going above it, before Go 1.19 you had only one knob to turn, the `GOGC` environment variable. This environment variable accept a relative target com,pared the current live heap size.
The default value for GOGC is 100 and meaning that the heap should double before GC should run again.

That works well for application that have small permanent live heaps, for example if your constant heap is just 50MB and you have 4GB available, you can double your heap targets any times before ever being in danger. 
If application load increases and temporary allocation increase, the dynamic targets would be 100MB, 200MB, 400MB, 800MB, 1600MB, and 3200MB. The load must double seven times to cross the 4GB mark, here running out of memory is extremely unlikely.

But now think back to our cache application example with a permanent 2GB live heap on 4GB machine. Event the forts doubling of the heap is highly problematic because the new target (4GB) would already reach the limit of the physical memory on the machine.



### Be less aggressive when enough memory available, be very aggressive when less memory available


## GOMEMLIMIT

### Soft limit



## Without GOMEMLIMT



## With GOMEMLIMIT



## Conclusion