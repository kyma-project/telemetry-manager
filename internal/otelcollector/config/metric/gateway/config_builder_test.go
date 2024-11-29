package gateway

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")

		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, actualExporterConfig.OTLP.Endpoint)

		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST")
		require.Equal(t, "http://localhost", string(envVars["OTLP_ENDPOINT_TEST"]))
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, actualExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-insecure").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")

		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-basic-auth").WithOTLPOutput(testutils.OTLPBasicAuth("user", "password")).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")

		actualExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := actualExporterConfig.OTLP.Headers
		authHeader, existing := headers["Authorization"]
		require.True(t, existing)
		require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)

		require.Contains(t, envVars, "BASIC_AUTH_HEADER_TEST_BASIC_AUTH")

		expectedBasicAuthHeader := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("user:password")))
		require.Equal(t, expectedBasicAuthHeader, string(envVars["BASIC_AUTH_HEADER_TEST_BASIC_AUTH"]))
	})

	t.Run("custom header", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-custom-header").WithOTLPOutput(testutils.OTLPCustomHeader("Authorization", "TOKEN_VALUE", "Api-Token")).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-custom-header")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test-custom-header"]
		headers := otlpExporterConfig.OTLP.Headers
		customHeader, exists := headers["Authorization"]
		require.True(t, exists)
		require.Equal(t, "${HEADER_TEST_CUSTOM_HEADER_AUTHORIZATION}", customHeader)

		require.Contains(t, envVars, "HEADER_TEST_CUSTOM_HEADER_AUTHORIZATION")
		require.Equal(t, "Api-Token TOKEN_VALUE", string(envVars["HEADER_TEST_CUSTOM_HEADER_AUTHORIZATION"]))
	})

	t.Run("mtls", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-mtls").WithOTLPOutput(testutils.OTLPClientTLSFromString("ca", "cert", "key")).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-mtls")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test-mtls"]
		require.Equal(t, "${OTLP_TLS_CERT_PEM_TEST_MTLS}", otlpExporterConfig.OTLP.TLS.CertPem)
		require.Equal(t, "${OTLP_TLS_KEY_PEM_TEST_MTLS}", otlpExporterConfig.OTLP.TLS.KeyPem)

		require.Contains(t, envVars, "OTLP_TLS_CERT_PEM_TEST_MTLS")
		require.Equal(t, "cert", string(envVars["OTLP_TLS_CERT_PEM_TEST_MTLS"]))

		require.Contains(t, envVars, "OTLP_TLS_KEY_PEM_TEST_MTLS")
		require.Equal(t, "key", string(envVars["OTLP_TLS_KEY_PEM_TEST_MTLS"]))
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.NotEmpty(t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		metricreaders := []config.MetricReader{
			{
				Pull: config.PullMetricReader{
					Exporter: config.MetricExporter{
						Prometheus: config.PrometheusMetricExporter{
							Host: "${MY_POD_IP}",
							Port: ports.Metrics,
						},
					},
				},
			},
		}

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "json", collectorConfig.Service.Telemetry.Logs.Encoding)
		require.Equal(t, metricreaders, collectorConfig.Service.Telemetry.Metrics.Readers)
	})

	t.Run("single pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)
		require.Equal(t, maxQueueSize, collectorConfig.Exporters["otlp/test"].OTLP.SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-3").Build(),
			},
			BuildOptions{},
		)
		
		require.NoError(t, err)
		expectedQueueSize := 85
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-1"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-2"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-3"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("exporters names", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-3").Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-3")
	})

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name           string
			goldenFileName string
			withOTLPInput  bool
		}{
			{
				name:           "OTLP Endpoint enabled",
				goldenFileName: "config.yaml",
				withOTLPInput:  true,
			},
			{
				name:           "OTLP Endpoint disabled",
				goldenFileName: "config_otlp_disabled.yaml",
				withOTLPInput:  false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config, _, err := sut.Build(
					context.Background(),
					[]telemetryv1alpha1.MetricPipeline{
						testutils.NewMetricPipelineBuilder().
							WithName("test").
							WithOTLPInput(tt.withOTLPInput).
							WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
					},
					BuildOptions{},
				)
				require.NoError(t, err)

				configYAML, err := yaml.Marshal(config)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.NoError(t, err)
				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
