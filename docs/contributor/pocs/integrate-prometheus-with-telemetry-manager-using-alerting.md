# Integrate Prometheus With Telemetry Manager Using Alerting

## Goal

The goal of the Proof of Concept is to test integrating Prometheus into Telemetry Manager using Alerting.

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
          - telemetry-manager-alerts-webhook.kyma-system:9090
    
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
4. Create an endpoint in Telemetry Manager to be invoked by Prometheus:
   ```go
    reconcileTriggerChan := make(chan event.GenericEvent, 1024)
    go func() {
        handler := func(w http.ResponseWriter, r *http.Request) {
            body, readErr := io.ReadAll(r.Body)
            if readErr != nil {
                http.Error(w, "Error reading request body", http.StatusInternalServerError)
                return
            }
            defer r.Body.Close()
   
            // TODO: add more context about which objects have to reconciled
            reconcileTriggerChan <- event.GenericEvent{}
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
    func NewMetricPipelineReconciler(client client.Client, reconcileTriggerChan chan event.GenericEvent, reconciler *metricpipeline.Reconciler) *MetricPipelineReconciler {
        return &MetricPipelineReconciler{
            Client:     client,
            reconciler: reconciler,
            Client:      client,
            reconciler:  reconciler,
            reconcileTriggerChan: reconcileTriggerChan,
        }
    }
    
    // SetupWithManager sets up the controller with the Manager.
    func (r *MetricPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
        // We use `Watches` instead of `Owns` to trigger a reconciliation also when owned objects without the controller flag are changed.
        return ctrl.NewControllerManagedBy(mgr).
                For(&telemetryv1alpha1.MetricPipeline{}).
                WatchesRawSource(&source.Channel{Source: r.reconcileTriggerChan},
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
6. Query Prometheus alerts in the Reconcile function:
   ```go
    import (
        "context"
        "fmt"
        "time"
    
        "github.com/prometheus/client_golang/api"
        promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
        logf "sigs.k8s.io/controller-runtime/pkg/log"
    )
    
    const prometheusAPIURL = "http://prometheus-server.default:80"
    
    func queryAlerts(ctx context.Context) error {
        client, err := api.NewClient(api.Config{
            Address: prometheusAPIURL,
        })
        if err != nil {
            return fmt.Errorf("failed to create Prometheus client: %w", err)
        }
    
        v1api := promv1.NewAPI(client)
        ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
    
        start := time.Now()
        alerts, err := v1api.Alerts(ctx)
    
        if err != nil {
            return fmt.Errorf("failed to query Prometheus alerts: %w", err)
        }
    
        logf.FromContext(ctx).Info("Prometheus alert query succeeded!",
            "elapsed_ms", time.Since(start).Milliseconds(),
            "alerts", alerts)
        return nil
    }
   ```

7. Add a Kubernetes service for the alerts endpoint to the kustomize file:
   ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: manager-alerts-webhook
      namespace: system
    spec:
      ports:
        - name: webhook
          port: 9090
          targetPort: 9090
      selector:
        app.kubernetes.io/name: manager
        app.kubernetes.io/instance: telemetry
        kyma-project.io/component: controller
        control-plane: telemetry-manager
   ```
8. Whitelist the endpoint port (9090) in the manager network policy:
   ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: manager
    spec:
      podSelector:
        matchLabels:
          app.kubernetes.io/name: manager
          app.kubernetes.io/instance: telemetry
          kyma-project.io/component: controller
          control-plane: telemetry-manager
      policyTypes:
        - Ingress
      ingress:
        - from:
            - ipBlock:
                cidr: 0.0.0.0/0
          ports:
            - protocol: TCP
              port: 8080
            - protocol: TCP
              port: 8081
            - protocol: TCP
              port: 9443
            - protocol: TCP
              port: 9090
   ```
9. Deploy the modified Telemetry Manager:
   ```shell
    export IMG=$DEV_IMAGE_REPO
    make docker-build
    make docker-push
    make install
    make deploy
   ```
10. Intentionally break any scrape Target to fire the InstanceDown alert. Look at Telemetry Manager logs, you should see that Prometheus is pushing alerts via the endpoint, which triggers immediate reconciliation.
