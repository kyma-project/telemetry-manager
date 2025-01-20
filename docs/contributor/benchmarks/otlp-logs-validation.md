# OTel LogPipeline set-up validation

This file documents the process of validating the whole LogPipeline with OTLP output flow. It defines the setup, that consists of the manually deployed log agent, the already-implemented log gateway, and log generators using flog.

The scope is to performance test the agent, observing the resulting values (such as throughput, resource consumption, reaction to backpressure), and to compare the agent to the previous FluentBit-based setup.



## 1. Set-up configuration steps

### With Helm

``` bash
k apply -f telemetry-manager/config/samples/operator_v1alpha1_telemetry.yaml

// Execute knowledge-hub/scripts/create_cls_log_pipeline.sh with the corresponding environment variables 

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts

helm install -n kyma-system logging open-telemetry/opentelemetry-collector -f telemetry-manager/docs/contributor/pocs/assets/otel-log-agent-values.yaml
```

- To set up the log agent manually, run:

``` bash
k apply -f telemetry-manager/config/samples/operator_v1alpha1_telemetry.yaml

// Execute knowledge-hub/scripts/create_cls_log_pipeline.sh with the corresponding environment variables 

k apply -f ./otlp-logs-validation.yaml
```



## 2. Resulting Resources

### Agent ConfigMap (OTel Config)

See [OTLP Logs Validation YAML](./otlp-logs-validation.yaml)

#### Things to take into consideration (at implementation)
- Dynamically inclusion/exclusion of namespaces, based on LogPipeline spec attributes
- Exclude FluentBit container in OTel configuration and OTel container in FluentBit configuration
- `receivers/filelog/operators`: The copy body to `attributes.original` must be avoided if `dropLogRawBody` flag is enabled

### Agent DaemonSet

See [OTLP Logs Validation YAML](./otlp-logs-validation.yaml)

### How does checkpointing work

- By enabling the storeCheckpoint preset (Helm) the `file_storage` extension is activated in the receiver
- The `file_storage` has the path `/var/lib/otelcol`
- This is later mounted as a `hostPath` volume in the DaemonSet spec
- Also set in the `storage` property of the filelog receiver

> `storage` = The ID of a storage extension to be used to store file offsets. File offsets allow the receiver to pick up where it left off in the case of a collector restart. If no storage extension is used, the receiver will manage offsets in memory only.



## 3. Benchmarking and Performance Tests Results

Setup Configuration:
   ``` bash
k create ns prometheus
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install -n "prometheus" "prometheus" prometheus-community/kube-prometheus-stack -f hack/load-tests/values.yaml --set grafana.adminPassword=myPwd

k apply -f telemetry-manager/hack/load-tests/log-agent-test-setup.yaml
```

For executing the load tests, the generated logs have to be isolated, hence the following line should be replaced in the ConfigMap of the log agent:

``` yaml
receivers:
  filelog:
    # ...
    include:
    - /var/log/pods/*/*/*.log # replace with "/var/log/pods/log-load-test*/*flog*/*.log"
```

For the 🏋️‍♀️ Backpressure Scenario additionally apply:
``` bash
k apply -f telemetry-manager/hack/load-tests/log-backpressure-config.yaml
```

PromQL Queries:
``` sql
-- RECEIVED
round(sum(rate(otelcol_receiver_accepted_log_records{service="telemetry-log-agent-metrics"}[20m])))

-- EXPORTED
round(sum(rate(otelcol_exporter_sent_log_records{service="telemetry-log-agent-metrics"}[20m])))

-- QUEUE
avg(sum(otelcol_exporter_queue_size{service="telemetry-log-agent-metrics"}))

-- MEMORY
round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-log-agent"}[20m])) by (pod) / 1024 / 1024)

-- CPU
round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-log-agent"}[20m])) by (pod), 0.1)
```

### 📊 Benchmarking Session #1

| Icon | Meaning                                              |
| ---- | ---------------------------------------------------- |
| ⏳    | Full-test, involving the whole setup, usually 20 min |
| 🪲    | Debugging session, usually shorter, not so reliable  |
| 🏋️‍♀️    | Backpressure Scenario                                |
| ⭐️    | Best results observed (in a given scenario)          |

#### ⏳ 18 Dec 2024, 13:45 - 14:05 (20 min)
- **Generator:** 10 replicas x 10 MB
- **Agent:** no CPU limit, no queue
- **Results:**
  - Agent RECEIVED/EXPORTED: 6.06K
  - Agent Memory:
    - Pod1: 70
    - Pod2: 70
  - Agent CPU:
    - Pod1: 0.5
    - Pod2: 0.4
  - Gateway RECEIVED/EXPORTED: 6.09K
  - Gateway QUEUE: 0

#### ⏳ 18 Dec 2024, 14:08 - 14:28 (20 min)
- **Generator:** 20 replicas x 10 MB
- **Agent:** no CPU limit, no queue
- **Results:**
  - Agent RECEIVED/EXPORTED: 4.93K
  - Agent Memory:
    - Pod1: 71
    - Pod2: 72
  - Agent CPU:
    - Pod1: 0.5
    - Pod2: 0.4
  - Gateway RECEIVED/EXPORTED: 4.93K
  - Gateway QUEUE: 0 (max. 6 at some point)

#### ⏳ 18 Dec 2024, 14:50 - 15:10 (20 min)
- **Generator:** 10 replicas x 20 MB
- **Agent:** no CPU limit, no queue
- **Results:**
  - Agent RECEIVED/EXPORTED: 5.94K
  - Agent Memory:
    - Pod1: 76
    - Pod2: 81
  - Agent CPU:
    - Pod1: 0.5
    - Pod2: 0.5
  - Gateway RECEIVED/EXPORTED: 5.94K
  - Gateway QUEUE: 0

#### ⏳ 18 Dec 2024, 15:24 - 15:34 (10 min)
- **Generator:** 10 replicas x 10 MB
- **Agent:** with CPU limit (1), no queue
- **Results:**
  - Agent RECEIVED/EXPORTED: 8.9K
  - Agent Memory: 64/62
  - Agent CPU: 0.5/0.5
  - Gateway RECEIVED/EXPORTED: 8.9K
  - Gateway QUEUE: 0

#### 🏋️‍♀️⭐️ 18 Dec 2024, 15:36 - 15:56 (20 min) (backpressure scenario)
- **Generator:** 10 replicas x 10 MB
- **Agent:** with CPU limit (1), no queue
- **Results:**
  - Agent RECEIVED/EXPORTED: 6.8K
  - Agent Memory:
    - Pod1: 66
    - Pod2: 67
  - Agent CPU:
    - Pod1: 0.6
    - Pod2: 0.5
  - Gateway RECEIVED: 6.8K
  - Gateway EXPORTED: 256
  - Gateway QUEUE: 328
- **Remarks:**
  - Agent does not stop when gateway refuses logs (because backpressure does not backpropagate)
  - It slows down/stops in other scenarios (see bellow) => SUCCESS

#### 🪲 19 Dec 2024, Agent exports logs to a debug endpoint (5 min)
- no networking involved
- 12/14 log generators x 10 MB
  - 19.5K => ~20K
  - MEM: 43/47
  - CPU: 0.7/0.8

#### 🪲 19 Dec 2024, Agent exports logs directly to mock backend (5 min)
- networking, but avoiding gateway
- 10 log generators x 10 MB
  - 5.3K
  - MEM: 58/59
  - CPU: 0.4/0.5
- 12 log generators x 10 MB
  - not increasing

#### 🪲 19 Dec 2024, Agent exports logs directly to mock backend with batching processor (5 min)
- networking, but with batching mechanism in-place
- 10 log generators x 10 MB, batch size: 1024
  - 8.3K
  - MEM: 68/73
  - CPU: 0.5/0.6
- 12 log generators x 10 MB, batch size: 1024
  - starts decreasing (~7.5K)
- 10 log generators x 10 MB, batch size: 2048
  - ~9K
  - MEM: 74/79
  - CPU: 0.6/0.7

#### ⏳ 19 Dec 2024, 13:46 - 14:06 (20 min)
- **Generator:** 10 replicas x 10 MB
- **Agent:** with CPU limit (1), no queue, with batch processing (1024)
- **Results:**
  - Agent RECEIVED/EXPORTED: 8.46K
  - Gateway RECEIVED/EXPORTED: 8.46K
  - Agent Memory: 69/76
  - Agent CPU: 0.5/0.7
  - Gateway QUEUE: 0 (max 191)

#### ⏳ 19 Dec 2024, ??:?? - ??:?? (20 min)
- **Generator:** 10 replicas x 10 MB
- **Agent:** with CPU limit (1), no queue, with batch processing (2048)
- **Results:**
  - lower throughput as for the 1024 scenario

#### ⏳ 19 Dec 2024, 15:55 - 16:15 (20 min)
- **Agent:** with CPU limit (1), no queue, with batch processing (1024)
- **Mock Backend:** memory limit x2 (2048Mi)
- **Generator:** 10 replicas x 10 MB
  - **Results:**
    - Agent RECEIVED/EXPORTED: 8.18K
    - Gateway RECEIVED/EXPORTED: 8.18K
    - Agent Memory: 70/71
    - Agent CPU: 0.6/0.6
    - Gateway QUEUE: 0
- **Generator:** 12 replicas x 10 MB (16:18 - 16:35)
  - **Results:**
    - Agent RECEIVED/EXPORTED: 8.6k
    - Gateway RECEIVED/EXPORTED: 8.6k
    - Agent Memory: 73/74
    - Agent CPU: 0.7/0.6
    - Gateway QUEUE: 0
- **Generator:** 14 replicas x 10 MB (16:35 - 16:40)
  - **Results:**
    - Agent RECEIVED/EXPORTED: 7.54K
    - Gateway RECEIVED/EXPORTED: 7.54K
    - lower

#### ⏳ 19 Dec 2024, 16:50 - 17:10 (20 min)
- **Generator:** 12 replicas x 10 MB
- **Agent:** with CPU limit (1), no queue, with batch processing (2048)
- **Mock Backend:** memory limit x2 (2048Mi)
- **Results:**
  - Agent RECEIVED/EXPORTED: 8.1K
  - Gateway RECEIVED/EXPORTED: 8.11K
  - Agent Memory: 74/81
  - Agent CPU: 0.6/0.5
  - Gateway QUEUE: 0 (max 2)

#### 🪲 20 Dec 2024, Multiple agents loading the gateway (5 min)
- **Setup:** 10 nodes, 10 agents, 1 generator / node (DaemonSet)
- **Results (WITH BATCHING):**
  - Agent RECEIVED/EXPORTED: 61.5K => 6.1K / agent instance
  - Gateway RECEIVED/EXPORTED: 61.5K/29.5K => 30K/14.7K / gateway instance
  - Agent Memory: 61-68/agent
  - Agent CPU: 0.4-0.8/agent
  - Gateway QUEUE: 510 (max 512, full)
  - ~10% exporter failed enqueue logs
  - 0% receiver refused logs
  - 0% exporter send failed logs
- **Results (WITHOUT BATCHING):**
  - Agent RECEIVED/EXPORTED: 31.4K => 3.1K / agent instance
  - Gateway RECEIVED/EXPORTED: 31.4K => 11.4K / gateway instance
  - Agent Memory: 61-68/agent
  - Agent CPU: 0.4-0.5/agent
  - Gateway QUEUE: 0 (max 6)
  - 0% exporter failed enqueue logs
  - 0% receiver refused logs
  - 0% exporter send failed logs

### 📊 Benchmarking Session #2

#### ⏳ 15 Jan 2025, 12:31 - 12:51 (20 min)
- **Generator:** 10 replicas x 10 MB
- **Results:**
  - Agent RECEIVED/EXPORTED: 14.4K
  - Gateway RECEIVED/EXPORTED: 14.4K
  - Agent Memory: 74/69
  - Agent CPU: 0.9/0.8
  - Gateway QUEUE: 0

#### ⏳⭐️ 15 Jan 2025, 14:31 - 14:08 (20 min)
- Gateways on separate nodes
- **Generator:** 10 replicas x 10 MB
- **Results:**
  - Agent RECEIVED/EXPORTED: 15.7K
  - Gateway RECEIVED/EXPORTED: 15.7K
  - Agent Memory: 82/71
  - Agent CPU: 1/0.9
  - Gateway CPU: 0.6/0.6
  - Gateway Memory: 62/68
  - Gateway QUEUE: 0

#### 🪲 15 Jan 2025, Agent exports logs to a debug endpoint (5 min)
- no networking involved
- ~15K / agent => ~30K

#### Removing compression for the OTLP exporters (on both agent and gateway)
- boosts throughput in the 4 nodes scenario
- the change seemed to have no impact in the 2 nodes scenario

#### ⏳ 15 Jan 2025, ? - ? (20 min)
- Gateways on separate nodes
- Compression disabled for OTLP exporters (on both agent and gateway) (default: gzip)
- **Generator:** 20 replicas (new set-up)
- **Results:**
  - Agent RECEIVED/EXPORTED: 15.3K
  - Gateway RECEIVED/EXPORTED: 15.3K

#### ⏳⭐️ 16 Jan 2025, ~13:17 (20 min)
- Gateways on separate nodes
- No Istio
- **Generator:** 10 replicas
- **Results:**
  - Agent RECEIVED/EXPORTED: 18.8K
  - Gateway RECEIVED/EXPORTED: 18.8K
  - Agent Memory: 76/73
  - Agent CPU: 0.8/0.9
  - Gateway Memory: 69/27
  - Gateway CPU: 0.6/0.6
  - Gateway QUEUE: 1/0

#### ⏳⭐️ 16 Jan 2025, ~13:56 (20 min)
- No gateway involved, agent sending directly to mock backend
- With Istio
- **Generator:** 10 replicas
- **Results:**
  - Agent RECEIVED/EXPORTED: 19K
  - Agent Memory: 82/74
  - Agent CPU: 1.3/0.8

#### 🪲 17 Jan 2025, ~10:36
- 1 node
- No gateway involved, agent sending directly to mock backend
- With Istio
- Agent has everything removed (no processors)
- **Generator:** 5 replicas
- **Results (without batching):**
  - Agent RECEIVED/EXPORTED: 11.8K / instance
- **Results (with batching):**
  - Agent RECEIVED/EXPORTED: 14K / instance

#### 🪲 17 Jan 2025, ~11:48
- 1 node
- No gateway involved, agent sending directly to mock backend
- With Istio
- Agent has everything removed (no processors), then we incrementally add them
- **Generator:** 30 replicas (10m CPU limit)
- 📥 Debug Exporter:
  - **Results (without batching):**
    - Agent RECEIVED/EXPORTED: 16K / instance
  - **Results (with batching):**
    - Agent RECEIVED/EXPORTED: 22.4K / instance
  - **Results (batching + filestorage):**
    - Agent RECEIVED/EXPORTED: 20K / instance
- 📥 OTEL Exporter:
  - **Results (batching + filestorage):**
    - Agent RECEIVED/EXPORTED: 15K / instance
  - **Results (batching + filestorage + sending queue):**
    - Agent RECEIVED/EXPORTED: 15K / instance

#### 🪲 17 Jan 2025, ~13:16
- No gateway involved, agent sending directly to mock backend
- With Istio
- **2 nodes:**
  - **Generator:** 60 replicas (10m CPU limit)
  - Agent RECEIVED/EXPORTED: 28.6K
  - Agent Memory: 78/71
  - Agent CPU: 1.3/1.3
- **3 nodes:**
  - **Generator:** 90 replicas (10m CPU limit)
  - Agent RECEIVED/EXPORTED: 44.6K
  - Agent Memory: ~76-90
  - Agent CPU: ~1.3


## 4. Comparison with FluentBit setup
In the FluentBit setup, for the very same (initial) scenario (i.e. 10 generator replicas [old set-up] / 2 agents), the [load test](https://github.com/kyma-project/telemetry-manager/actions/runs/12691802471) outputs the following values for the agent:
- Exported Log Records/second: 27.8K


## 5. Conclusions
- Before 15 Jan. (first session):
  - A lower performance can be expected, compared to the FluentBit counterpart setup.
  - Backpressure is currently not backpropagated from the gateway to the agent, resulting in logs being queued/lost on the gateway end, since the agent has no way of knowing when to stop, thus exports data continuously. (This is a known issue, that should get solved by the OTel community in the next half year)
  - Agent slows down if the load is increased (i.e. more generators / more logs / more data).
  - The network communication between the agent and the gateway or/and the gateway represent a bottleneck in this setup, since when using just a debug endpoint as an exporter, higher throughput was observed.
  - CPU and Memory consumption are surprisingly low, and this was not improved by removing the limits (quite the opposite was observed, with the CPU throttling more often and the throughput decreasing).
  - When enabling the batch processor, throughput was increasing, but this comes at the cost of losing logs in some scenarios.
  - More/other methods of improving the throughput might still be worth investigating.
- After 15 jan. (second session):
  - Removing the gateway improves throughput
  - We now better understand the performance impact of each OTEL processor and of enabling/disabling compression
  - Generators configuration greatly influence the setup => more generators exporting less data and taking less CPU leads to higher throughput than fewer generators taking more CPU and exporting more data
  - There is a hard limit (see debug endpoint scenario) that we still not fully understand, since strictly based on the benchmarking numbers of OTEL, we should be getting higher throughput (i.e. something related to the infrastructure could be influencing this).
  - We have now a more performant setup configuration, being more comparable with the numbers from the FluentBit setup