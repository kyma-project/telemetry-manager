# Istio Integration

When you have the Istio module in your cluster, the Telemetry module automatically integrates with it. It detects the Istio installation and configures the Telemetry components accordingly, enabling secure mTLS communication for outbound data export by default.

## Receiving Data from Your Applications

The OTLP Gateway runs as a DaemonSet with one instance per cluster node. Its ingestion Service uses Kubernetes [internal traffic policy](https://kubernetes.io/docs/concepts/services-networking/service-traffic-policy/#using-service-internal-traffic-policy) set to `Local`, so telemetry data that your applications send is always received by the gateway instance on the same node.

Because the data path stays within a single node, Istio sidecars and mTLS are not required on the ingestion path. Applications can send data to the OTLP Gateway using a standard plain text connection regardless of whether they are part of the Istio mesh.

> [!TIP]
> Learn more about Istio-specific input configuration for logs, traces, and metrics:
>
> - Configure Istio Access Logs
> - Configure Istio Tracing
> - Collect Istio Metrics

![arch](./../assets/istio-input.drawio.svg)

## Node-Local Ingestion Security

The OTLP Gateway's node-local ingestion path provides the following security properties:

- **No cross-node traffic on ingestion**: The Service routes traffic only to the local node's gateway Pod. Telemetry data does not travel over the node-to-node network.
- **Kernel-level network isolation**: Pod-to-pod communication on the same node uses virtual network interfaces connected through a virtual bridge in the node's root network namespace. Intercepting this traffic requires root access to the node's network namespace or a man-in-the-middle position — both of which indicate a broader node compromise.
- **mTLS is not required for ingestion**: Because the data path stays within a single node, encryption with mTLS is not necessary for the push from applications to the gateway. Removing Istio from the ingestion path eliminates a performance bottleneck without reducing the security posture.

> [!NOTE]
> This applies only to the ingestion path (applications pushing data to the OTLP Gateway). For the export path (the OTLP Gateway sending data to your backend), mTLS is still used automatically when the backend is part of the Istio mesh. See [Sending Data to In-Cluster Backends](#sending-data-to-in-cluster-backends).

For the detailed investigation, see the [node-local traffic PoC](./../../contributor/pocs/node-local-traffic/node-local-traffic.md).

## Sending Data to In-Cluster Backends

The OTLP Gateway automatically secures the connection when sending data to your observability backends.

If you use an in-cluster backend that is part of the Istio mesh, the OTLP Gateway automatically uses mTLS to send data to the backend securely. You don't need any special configuration for this.

For sending data to backends outside the cluster, see [Integrate With Your OTLP Backend](./../integrate-otlp-backend/README.md).

![arch](./../assets/istio-output.drawio.svg)
