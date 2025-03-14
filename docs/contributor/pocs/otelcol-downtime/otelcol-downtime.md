# OpenTelemetry Collector Downtime PoC

This Proof of Concept (PoC) explores the behavior of OpenTelemetry (OTel) Collector clients — such as instrumented applications and Istio proxies — during collector downtime in a non-HA (High Availability) setup.

## OpenTelemetry SDK Behavior

Most clients sending data to an OTel Collector are applications instrumented with the OTel SDK. The OTel SDK specification clearly defines retry behavior:

- [Trace SDK Export](https://opentelemetry.io/docs/specs/otel/trace/sdk/#exportbatch)
- [Metrics SDK Export](https://opentelemetry.io/docs/specs/otel/metrics/sdk/#exportbatch)
- [Logs SDK Export](https://opentelemetry.io/docs/specs/otel/logs/sdk/#export)

According to the [OTLP specification](https://opentelemetry.io/docs/specs/otlp/), the client should retry in the following situations:  

1. **Retryable errors** indicated by specific gRPC or HTTP response status codes. This occurs, for example, when the server is temporarily unable to process the data.  
2. **Connection failures**, where the client cannot establish a connection to the server. In this case, the client should retry using an exponential backoff strategy with randomized jitter between retries.  

However, based on our tests with the Java and Go OTel SDKs, retries currently only occur in scenario **(1)** but not in **(2)**. This is a critical limitation, as scenario **(2)** is particularly relevant when the collector experiences downtime.  

That said, since the behavior is explicitly stated in the specification, we are confident that it will be implemented in future versions of the SDKs.  

## How to Test  

### 1. Set Up Environment  

To provision the environment, run the following command from the root directory of the repository:  

```bash
make provision-k3d
```

### 2. Deploy Required Resources  

Apply the necessary resources:  

```bash
kubectl apply -f ./go-otel-sdk-retries.yaml
```

### 3. Send Traces and Verify Behavior  

#### a) Initial Trace Verification  

1. **Port forward** into a pod labeled with `trace-gen` in the `trace-gen` namespace:  

   ```bash
   kubectl port-forward -n trace-gen pod/$(kubectl get pod -n trace-gen -l app=trace-gen -o jsonpath="{.items[0].metadata.name}") 8080:8080
   ```  

2. **Send a trace** by triggering a request:  

   ```bash
   curl localhost:8080/terminate
   ```  

3. **Check logs of the trace sink** to verify that traces are received:  

   ```bash
   kubectl logs -n trace-sink deployment/trace-sink
   ```

   You should see traces successfully received.  

#### b) Simulating Collector Downtime  

1. **Scale down the trace sink to zero replicas** to simulate collector unavailability:  

   ```bash
   kubectl scale deployment -n trace-sink trace-sink --replicas=0
   ```  

2. **Send another trace request**:  

   ```bash
   curl localhost:8080/terminate
   ```  

3. **Wait 30 seconds** to allow for potential retries.  

4. **Scale the trace sink back up**:  

   ```bash
   kubectl scale deployment -n trace-sink trace-sink --replicas=1
   ```  

5. **Check the trace sink logs** again:  

   ```bash
   kubectl logs -n trace-sink deployment/trace-sink
   ```  

6. **Verify if spans were sent after recovery.**  

   - **Expected behavior (currently not happening):** The spans should be delivered after the trace sink becomes available again.  
   - **Current behavior:** Spans are lost when the collector is unavailable, indicating that no retries occur in this scenario.  

---

### 4. Compare Results  

Compare the behavior of traces sent when the trace sink is available versus when it experiences downtime. This test confirms whether spans are retried and eventually delivered after recovery.  

---

Let me know if this structure works for you! 🚀



## Istio Proxies  

Istio proxies can send access logs and spans to an OTLP endpoint, but they do not appear to use the OpenTelemetry (OTel) SDK. However, Envoy provides a way to configure retry policies:  

- [Envoy OpenTelemetry Trace Configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/trace/v3/opentelemetry.proto.html)  
- [Envoy OpenTelemetry Access Logger Configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/open_telemetry/v3/logs_service.proto)  

These configurations are not reflected in Istio's mesh configuration by default. However, enabling them is relatively straightforward. A feature request similar to [this one](https://github.com/istio/istio/issues/52873) could be submitted to improve support for retry policies.  

Additionally, tests indicate that Istio proxies currently do not implement any retry functionality. As a result, if the collector is unavailable, data is dropped.  
