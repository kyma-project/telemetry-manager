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

		require.Contains(t, collectorConfig.Exporters, "otlp")
		actualExporterConfig := collectorConfig.Exporters["otlp"]
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})

			require.Contains(t, collectorConfig.Exporters, "otlp")
			actualExporterConfig := collectorConfig.Exporters["otlp"]
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

	t.Run("marshaling", func(t *testing.T) {
		expected := `receivers:
    kubeletstats:
        collection_interval: 30s
        auth_type: serviceAccount
        endpoint: https://${env:MY_NODE_NAME}:10250
        insecure_skip_verify: true
        metric_groups:
            - container
            - pod
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
processors: {}
extensions:
    health_check:
        endpoint: ${MY_POD_IP}:13133
service:
    pipelines:
        metrics:
            receivers:
                - kubeletstats
            processors: []
            exporters:
                - otlp
    telemetry:
        metrics:
            address: ${MY_POD_IP}:8888
        logs:
            level: info
    extensions:
        - health_check
`

		collectorConfig := MakeConfig(gatewayServiceName, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build()})

		yamlBytes, err := yaml.Marshal(collectorConfig)
		require.NoError(t, err)
		require.Equal(t, expected, string(yamlBytes))
	})
}
