# Integrate Prometheus with Telemetry Manager using Alerting

## Goal

The goal of the PoC is to have a corase-grained implementation of integrating Prometheus into Telemetry Manager using Alerting

## Setup

1. Create a Kubernetes cluster (k3d or Gardener)
2. Create an overrides file for Prometheus Helm Chart and call it `overrides.yaml`
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
3. Deploy Prometheus
```shell
kubectl create ns prometheus
helm install -f overrides.yaml  prometheus prometheus-community/prometheus
```
