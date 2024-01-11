# Integrate Prometheus with Telemetry Manager using Alerting

## Goal

The goal of the Proof of Concept is to test out integrating Prometheus into Telemetry Manager using Alerting.

## Setup

Follow these steps to set up the required environment:

1. Create a Kubernetes cluster (k3d or Gardener).
2. Create an overrides file specifically for the Prometheus Helm Chart. Save the file as `overrides.yaml`.
```yaml
alertmanager:
  enabled: false

prometheus-pushgateway:
  enabled: false

prometheus-node-exporter:
  enabled: false

server:  
  alertmanagers:
  - static_configs:
    - targets:
      - telemetry-operator-alerts-webhook.kyma-system:9090

serverFiles:
  alerting_rules.yml:
   groups:
     - name: Instances
       rules:
         - alert: InstanceDown
           expr: up == 0
           for: 5m
           labels:
             severity: page
           annotations:
             description: '{{ $labels.instance }} of job {{ $labels.job }} has been down for more than 5 minutes.'
             summary: 'Instance {{ $labels.instance }} down' 
  prometheus.yml:
    rule_files:
      - /etc/config/recording_rules.yml
      - /etc/config/alerting_rules.yml

    scrape_configs:
      - job_name: prometheus
        static_configs:
          - targets:
            - localhost:9090

      - job_name: 'kubernetes-service-endpoints'
        honor_labels: true
        kubernetes_sd_configs:
          - role: endpoints
        relabel_configs:
          - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
            action: keep
            regex: true
          - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape_slow]
            action: drop
            regex: true
          - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
            action: replace
            target_label: __scheme__
            regex: (https?)
          - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
            action: replace
            target_label: __metrics_path__
            regex: (.+)
          - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
            action: replace
            target_label: __address__
            regex: (.+?)(?::\d+)?;(\d+)
            replacement: $1:$2
          - action: labelmap
            regex: __meta_kubernetes_service_annotation_prometheus_io_param_(.+)
            replacement: __param_$1
          - action: labelmap
            regex: __meta_kubernetes_service_label_(.+)
          - source_labels: [__meta_kubernetes_namespace]
            action: replace
            target_label: namespace
          - source_labels: [__meta_kubernetes_service_name]
            action: replace
            target_label: service
          - source_labels: [__meta_kubernetes_pod_node_name]
            action: replace
            target_label: node

```
3. Deploy Prometheus.
```shell
kubectl create ns prometheus
helm install -f overrides.yaml  prometheus prometheus-community/prometheus
```
4. Implement an endpoint to be called by Prometheus in the Telemetry Manager by copying the following snippet into `main.go`:
```go
	alertEvents := make(chan event.GenericEvent, 1024)
	go func() {
		handler := func(w http.ResponseWriter, r *http.Request) {
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
				return
			}
			defer r.Body.Close()

			mutex.Lock()
			setupLog.Info("Webhook was called", "req", string(body))
			mutex.Unlock()

			alertEvents <- event.GenericEvent{}

			w.WriteHeader(http.StatusOK)
		}
		
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v2/alerts", handler)

		server := &http.Server{
			Addr:              ":9090",
			ReadHeaderTimeout: 10 * time.Second,
			Handler:           mux,
		}

		if serverErr := server.ListenAndServe(); serverErr != nil {
			mutex.Lock()
			setupLog.Error(serverErr, "Cannot start webhook server")
			mutex.Unlock()
		}
	}()
```
5. Trigger reconciliation in MetricPipelineController whenever the endpoint is called by Prometheus:
```go
func NewMetricPipelineReconciler(client client.Client, alertEvents chan event.GenericEvent, reconciler *metricpipeline.Reconciler) *MetricPipelineReconciler {
	return &MetricPipelineReconciler{
		Client:     client,
		reconciler: reconciler,
		Client:      client,
		reconciler:  reconciler,
		alertEvents: alertEvents,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // We use `Watches` instead of `Owns` to trigger a reconciliation also when owned objects without the controller flag are changed.
    return ctrl.NewControllerManagedBy(mgr).
            For(&telemetryv1alpha1.MetricPipeline{}).
            WatchesRawSource(&source.Channel{Source: r.alertEvents},
            handler.EnqueueRequestsFromMapFunc(r.mapPrometheusAlertEvent)).
		...
}

func (r *MetricPipelineReconciler) mapPrometheusAlertEvent(ctx context.Context, _ client.Object) []reconcile.Request {
    logf.FromContext(ctx).Info("Handling Prometheus alert event")
    requests, err := r.createRequestsForAllPipelines(ctx)
    if err != nil {
    logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
    }
    return requests
}
```
