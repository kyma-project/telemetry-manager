Tests performed

1.  Traces (with batching) without istio:

    - Telemetrygen Replicas: 6
    - Telemetry trace agent: 1
    - Telemetry sink: 1
    - Throughout: 62k

````
➜ k top pods -n trace-sink
NAME               CPU(cores)   MEMORY(bytes)
trace-sink-fhvwc   2129m        257Mi

~ using ☁️  default/
➜ k top pods -n trace-load-test
NAME                                   CPU(cores)   MEMORY(bytes)
trace-load-generator-c6548c97d-f5nk4   1109m        48Mi
trace-load-generator-c6548c97d-gwcfz   1100m        69Mi
trace-load-generator-c6548c97d-hvbv6   1119m        49Mi
trace-load-generator-c6548c97d-rhf5b   1110m        46Mi
trace-load-generator-c6548c97d-z47ld   1106m        56Mi
trace-load-generator-c6548c97d-zjz2j   1112m        54Mi

~ using ☁️  default/
➜ k top pods -n kyma-system
NAME                          CPU(cores)   MEMORY(bytes)
telemetry-trace-agent-jczwh   4400m        214Mi


➜ k top node
NAME                                                  CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
shoot--berlin--rg-large-worker-qs9e3-z1-65745-xdsjl   13110m       82%      2555Mi          4%
```



2a. Logs (without batching) without Istio, Payload: Body = `{“foo":"bar"} `
   - Telemetrygen Replicas: 6
    - Telemetry trace agent: 1
    - Telemetry sink: 1
    - Throughout: 21k
    ➜ k top node
NAME                                                  CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
shoot--berlin--rg-large-worker-qs9e3-z1-65745-6flqg   13336m       83%      2893Mi          4%



➜ k top pod -n kyma-system
NAME                                     			  	CPU(cores)   MEMORY(bytes)
istio-controller-manager-74f4b8f74-h4p8v   		2m           	41Mi
telemetry-logs-agent-cgk5x                 			6590m        	191Mi


➜ k top pod -n log-sink
NAME             CPU(cores)   MEMORY(bytes)
log-sink-hd4z5   345m         75Mi


➜ k top pod -n log-load-test
NAME                                  CPU(cores)   MEMORY(bytes)
log-load-generator-7f48fd849d-2qtbr   799m         18Mi
log-load-generator-7f48fd849d-dxsrr   817m         19Mi
log-load-generator-7f48fd849d-hktrz   802m         20Mi
log-load-generator-7f48fd849d-plddl   808m         21Mi
log-load-generator-7f48fd849d-rjkth   799m         19Mi
log-load-generator-7f48fd849d-rnl2v   793m         20Mi
log-load-generator-7f48fd849d-w6z5f   795m         21Mi
log-load-generator-7f48fd849d-zr6j6   809m         20Mi


2b. Logs (without batching) without Istio, Payload: Body = `{“foo":"bar"} ` with 32 CPU 
   - Telemetrygen Replicas: 6
    - Telemetry trace agent: 1
    - Telemetry sink: 1
    - Throughout: 30k/34k(with 10 replicas)
    
➜ k top node
NAME                                                   CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
shoot--berlin--rg-xlarge-worker-zzzq4-z1-6dd68-xd7sg   23947m       75%      2460Mi          2%

telemetry-manager on  main [⇡$!?] via 🐹 v1.25.3 on 🐳 v28.5.1 using ☁️  default/
➜ k top pods -n kyma-system
NAME                              CPU(cores)   MEMORY(bytes)
telemetry-trace-log-agent-wtzqr   12804m       170Mi

telemetry-manager on  main [⇡$!?] via 🐹 v1.25.3 on 🐳 v28.5.1 using ☁️  default/
➜ k top pods -n log-load-test
NAME                                  CPU(cores)   MEMORY(bytes)
log-load-generator-65595cfd54-4rp46   1928m        30Mi
log-load-generator-65595cfd54-7kk9t   1914m        28Mi
log-load-generator-65595cfd54-958fh   1930m        28Mi
log-load-generator-65595cfd54-d4l72   1914m        29Mi
log-load-generator-65595cfd54-p7wjv   1925m        32Mi
log-load-generator-65595cfd54-sdb9r   1939m        26Mi

telemetry-manager on  main [⇡$!?] via 🐹 v1.25.3 on 🐳 v28.5.1 using ☁️  default/
➜ k top pods -n telemetry-sink
NAME                   CPU(cores)   MEMORY(bytes)
telemetry-sink-q9z27   510m         118Mi


3. Logs (without batching) with Istio, Payload: Body = `{“foo":"bar"} `
   - Telemetrygen Replicas: 6
    - Telemetry trace agent: 1
    - Telemetry sink: 1
    - Throughout: 4.5k

    ➜ k top nodes
NAME                                                  					CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
shoot--berlin--rg-large-worker-qs9e3-z1-65745-6flqg   5820m        36%      			3160Mi          5%


➜ k top pod -n log-sink
NAME             CPU(cores)   MEMORY(bytes)
log-sink-r642h   35m          64Mi

➜ k top pod -n log-load-test
NAME                                  CPU(cores)   MEMORY(bytes)
log-load-generator-5f868979ff-4zxq7   440m         60Mi
log-load-generator-5f868979ff-8jsw7   431m         59Mi
log-load-generator-5f868979ff-96vjq   430m         61Mi
log-load-generator-5f868979ff-j47r5   432m         60Mi
log-load-generator-5f868979ff-mqlqt   433m         59Mi
log-load-generator-5f868979ff-rfzdc   435m         60Mi
log-load-generator-5f868979ff-v2z28   439m         57Mi
log-load-generator-5f868979ff-w8vq5   434m         61Mi

NAME                                       CPU(cores)   MEMORY(bytes)
istio-controller-manager-74f4b8f74-h4p8v   2m           41Mi
telemetry-logs-agent-mg9v2                 2025m        259Mi

4. Logs (without batching) with Istio (used appProtocol tcp), Payload: Body = `{“foo":"bar"} `
   - Telemetrygen Replicas: 6
    - Telemetry trace agent: 1
    - Telemetry sink: 1
    - Throughout: 14k

    ➜ k top nodes
NAME                                                  					CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
shoot--berlin--rg-large-worker-qs9e3-z1-65745-6flqg   5820m        36%      			3160Mi          5%


➜ k top pod -n log-sink
NAME             CPU(cores)   MEMORY(bytes)
log-sink-r642h   35m          64Mi

➜ k top pod -n log-load-test
NAME                                  CPU(cores)   MEMORY(bytes)
log-load-generator-5f868979ff-4zxq7   440m         60Mi
log-load-generator-5f868979ff-8jsw7   431m         59Mi
log-load-generator-5f868979ff-96vjq   430m         61Mi
log-load-generator-5f868979ff-j47r5   432m         60Mi
log-load-generator-5f868979ff-mqlqt   433m         59Mi
log-load-generator-5f868979ff-rfzdc   435m         60Mi
log-load-generator-5f868979ff-v2z28   439m         57Mi
log-load-generator-5f868979ff-w8vq5   434m         61Mi

NAME                                       CPU(cores)   MEMORY(bytes)
istio-controller-manager-74f4b8f74-h4p8v   2m           41Mi
telemetry-logs-agent-mg9v2    



4. 1.  Traces (with batching) without istio: 

    - Telemetrygen Replicas: 6
    - Telemetry trace agent: 1
    - Telemetry sink: 1
    - Throughout: 101k

➜ k top node
NAME                                                   CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
shoot--berlin--rg-xlarge-worker-zzzq4-z1-6dd68-xd7sg   3889m        12%      2519Mi          2%

➜ k top pods -n kyma-system
NAME                              CPU(cores)   MEMORY(bytes)
telemetry-trace-log-agent-wtzqr   1004m        206Mi

➜ k top pods -n trace-load-test
NAME                                    CPU(cores)   MEMORY(bytes)
trace-load-generator-85b5f9b7f8-gdvdd   419m         30Mi
trace-load-generator-85b5f9b7f8-jscnd   419m         28Mi
trace-load-generator-85b5f9b7f8-pmcj6   422m         31Mi
trace-load-generator-85b5f9b7f8-qghcj   420m         36Mi
trace-load-generator-85b5f9b7f8-r766c   422m         30Mi
trace-load-generator-85b5f9b7f8-xtphv   414m         33Mi

➜ k top pods -n telemetry-sink
NAME                   CPU(cores)   MEMORY(bytes)
telemetry-sink-q9z27   552m         126Mi




/increased to 10 replicas


| Test # | Signal Type | Telemetrygen Replicas | Trace Agent | Trace Sink | Throughput   | Istio     | Node size CPU |Node CPU Usage | Avg Load Generator CPU | Telemetry Agent CPU |
|--------|-------------|----------------------|-------------|------------|--------------|------------|----------------|----------------|-----------------------|---------------------|
| 1      | Traces      | 6                    | 1           | 1          | 62k          | No         | 16             |13110m          | 1112m                 | 4400m               |
| 4.1    | Traces      | 6                    | 1           | 1          | 101k         | No         | 32             |3889m          | 419m                  | 1004m                |
| 2a     | Logs        | 6                    | 1           | 1          | 21k          | No         | 16             |13336m         | 802m                  | 6590m                |
| 2b     | Logs        | 6                    | 1           | 1          | 30k/34k      | No         | 32             | 23947m               | 1925m          | 12804m               |
| 3      | Logs        | 6                    | 1           | 1          | 4.5k         | Yes        | 16         | 5820m          | 433m                  | 2025m                   |
| 4      | Logs        | 6                    | 1           | 1          | 14k          | Yes (tcp)  | 16         |5820m          | 433m                  | n/a                      |


