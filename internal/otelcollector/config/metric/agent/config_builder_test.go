package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMakeAgentConfig(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})

		actualExporterConfig := collectorConfig.Exporters.OTLP
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})

			actualExporterConfig := collectorConfig.Exporters.OTLP
			require.True(t, actualExporterConfig.TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		t.Run("no input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			})

			require.Nil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("runtime input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("prometheus input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/self", "prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-prometheus"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("istio input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithIstioInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})

		t.Run("multiple input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).WithPrometheusInputOn(true).WithIstioInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 3)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/self", "prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-prometheus"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		t.Run("no pipeline has input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
				testutils.NewMetricPipelineBuilder().Build(),
			})

			require.Nil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("some pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(false).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("all pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("some pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(false).Build(),
				testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/self", "prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-prometheus"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("all pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(true).Build(),
				testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/self", "prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-prometheus"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("multiple input types enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(true).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
			})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 2)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/self", "prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"resource/delete-service-name", "resource/insert-input-source-prometheus"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})
	})

	t.Run("marshaling", func(t *testing.T) {
		expected := `extensions:
    health_check:
        endpoint: ${MY_POD_IP}:13133
service:
    pipelines:
        metrics/istio:
            receivers:
                - prometheus/istio
            processors:
                - resource/delete-service-name
                - resource/insert-input-source-istio
            exporters:
                - otlp
        metrics/prometheus:
            receivers:
                - prometheus/self
                - prometheus/app-pods
                - prometheus/app-services
            processors:
                - resource/delete-service-name
                - resource/insert-input-source-prometheus
            exporters:
                - otlp
        metrics/runtime:
            receivers:
                - kubeletstats
            processors:
                - resource/delete-service-name
                - resource/insert-input-source-runtime
            exporters:
                - otlp
    telemetry:
        metrics:
            address: ${MY_POD_IP}:8888
        logs:
            level: info
    extensions:
        - health_check
receivers:
    kubeletstats:
        collection_interval: 30s
        auth_type: serviceAccount
        endpoint: https://${env:MY_NODE_NAME}:10250
        insecure_skip_verify: false
        metric_groups:
            - container
            - pod
    prometheus/self:
        config:
            scrape_configs:
                - job_name: opentelemetry-collector
                  scrape_interval: 30s
                  static_configs:
                    - targets:
                        - ${MY_POD_IP}:8888
    prometheus/app-pods:
        config:
            scrape_configs:
                - job_name: app-pods
                  scrape_interval: 30s
                  relabel_configs:
                    - source_labels: [__meta_kubernetes_pod_node_name]
                      regex: $MY_NODE_NAME
                      action: keep
                    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
                      regex: "true"
                      action: keep
                    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scheme]
                      regex: (https?)
                      target_label: __scheme__
                      action: replace
                    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
                      regex: (.+)
                      target_label: __metrics_path__
                      action: replace
                    - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
                      regex: ([^:]+)(?::\d+)?;(\d+)
                      target_label: __address__
                      replacement: $$1:$$2
                      action: replace
                    - source_labels: [__meta_kubernetes_pod_phase]
                      regex: Pending|Succeeded|Failed
                      action: drop
                    - source_labels: [__meta_kubernetes_pod_container_init]
                      regex: (true)
                      action: drop
                    - source_labels: [__meta_kubernetes_pod_container_name]
                      regex: (istio-proxy)
                      action: drop
                  kubernetes_sd_configs:
                    - role: pod
    prometheus/app-services:
        config:
            scrape_configs:
                - job_name: app-services
                  scrape_interval: 30s
                  relabel_configs:
                    - source_labels: [__meta_kubernetes_endpoint_node_name]
                      regex: $MY_NODE_NAME
                      action: keep
                    - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
                      regex: "true"
                      action: keep
                    - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
                      regex: (https?)
                      target_label: __scheme__
                      action: replace
                    - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
                      regex: (.+)
                      target_label: __metrics_path__
                      action: replace
                    - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
                      regex: ([^:]+)(?::\d+)?;(\d+)
                      target_label: __address__
                      replacement: $$1:$$2
                      action: replace
                    - source_labels: [__meta_kubernetes_pod_phase]
                      regex: Pending|Succeeded|Failed
                      action: drop
                    - source_labels: [__meta_kubernetes_pod_container_init]
                      regex: (true)
                      action: drop
                    - source_labels: [__meta_kubernetes_pod_container_name]
                      regex: (istio-proxy)
                      action: drop
                    - source_labels: [__meta_kubernetes_service_name]
                      target_label: service
                      action: replace
                  kubernetes_sd_configs:
                    - role: endpoints
    prometheus/istio:
        config:
            scrape_configs:
                - job_name: istio-proxy
                  scrape_interval: 30s
                  metrics_path: /stats/prometheus
                  relabel_configs:
                    - source_labels: [__meta_kubernetes_pod_node_name]
                      regex: $MY_NODE_NAME
                      action: keep
                    - source_labels: [__meta_kubernetes_pod_container_name]
                      regex: istio-proxy
                      action: keep
                    - source_labels: [__meta_kubernetes_pod_container_port_name]
                      regex: http-envoy-prom
                      action: keep
                    - source_labels: [__meta_kubernetes_pod_phase]
                      regex: Pending|Succeeded|Failed
                      action: drop
                  metric_relabel_configs:
                    - source_labels: [__name__]
                      regex: istio_.*
                      action: keep
                  kubernetes_sd_configs:
                    - role: pod
processors:
    resource/delete-service-name:
        attributes:
            - action: delete
              key: service.name
    resource/insert-input-source-runtime:
        attributes:
            - action: insert
              key: kyma.source
              value: runtime
    resource/insert-input-source-prometheus:
        attributes:
            - action: insert
              key: kyma.source
              value: prometheus
    resource/insert-input-source-istio:
        attributes:
            - action: insert
              key: kyma.source
              value: istio
exporters:
    otlp:
        endpoint: metrics.telemetry-system.svc.cluster.local:4317
        tls:
            insecure: true
        sending_queue:
            enabled: true
            queue_size: 512
        retry_on_failure:
            enabled: true
            initial_interval: 5s
            max_interval: 30s
            max_elapsed_time: 300s
`

		collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).WithPrometheusInputOn(true).WithIstioInputOn(true).Build(),
		})

		yamlBytes, err := yaml.Marshal(collectorConfig)

		require.NoError(t, err)
		require.Equal(t, expected, string(yamlBytes))
	})
}
