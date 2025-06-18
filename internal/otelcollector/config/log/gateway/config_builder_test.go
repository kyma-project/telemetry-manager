package gateway

import (
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

func TestBuildConfig(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
		},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)

		const endpointEnvVar = "OTLP_ENDPOINT_TEST"
		expectedEndpoint := fmt.Sprintf("${%s}", endpointEnvVar)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, otlpExporterConfig.OTLP.Endpoint)

		require.Contains(t, envVars, endpointEnvVar)
		require.Equal(t, "http://localhost", string(envVars[endpointEnvVar]))
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, otlpExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-insecure").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
			ModuleVersion: "1.0.0",
		})
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")

		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-basic-auth").WithOTLPOutput(testutils.OTLPBasicAuth("user", "password")).Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := otlpExporterConfig.OTLP.Headers
		authHeader, existing := headers["Authorization"]
		require.True(t, existing)
		require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)

		require.Contains(t, envVars, "BASIC_AUTH_HEADER_TEST_BASIC_AUTH")

		expectedBasicAuthHeader := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("user:password")))
		require.Equal(t, expectedBasicAuthHeader, string(envVars["BASIC_AUTH_HEADER_TEST_BASIC_AUTH"]))
	})

	t.Run("custom header", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-custom-header").WithOTLPOutput(testutils.OTLPCustomHeader("Authorization", "TOKEN_VALUE", "Api-Token")).Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
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
		collectorConfig, envVars, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-mtls").WithOTLPOutput(testutils.OTLPClientTLSFromString("ca", "cert", "key")).Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
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
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.NotEmpty(t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
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
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)
		require.Equal(t, maxQueueSize, collectorConfig.Exporters["otlp/test"].OTLP.SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-1").WithOTLPOutput().Build(),
			testutils.NewLogPipelineBuilder().WithName("test-2").WithOTLPOutput().Build(),
			testutils.NewLogPipelineBuilder().WithName("test-3").WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})

		require.NoError(t, err)

		expectedQueueSize := 85 // Total queue size (256) divided by the number of pipelines (3)
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-1"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-2"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-3"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Service.Pipelines, "logs/test")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test"].Receivers, "otlp")

		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[1], "transform/set-observed-time-if-zero")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[2], "istio_noise_filter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[3], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[4], "resource/insert-cluster-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[5], "service_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[6], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[7], "istio_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[8], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines["logs/test"].Exporters, "otlp/test")
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-1").WithOTLPOutput().Build(),
			testutils.NewLogPipelineBuilder().WithName("test-2").WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")

		require.Contains(t, collectorConfig.Service.Pipelines, "logs/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-1"].Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-1"].Receivers, "otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[1], "transform/set-observed-time-if-zero")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[2], "istio_noise_filter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[3], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[4], "resource/insert-cluster-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[5], "service_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[6], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[7], "istio_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-1"].Processors[8], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines, "logs/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-2"].Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-2"].Receivers, "otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[1], "transform/set-observed-time-if-zero")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[2], "istio_noise_filter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[3], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[4], "resource/insert-cluster-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[5], "service_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[6], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[7], "istio_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-2"].Processors[8], "batch")

		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_1")
		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_2")
	})

	t.Run("marshaling", func(t *testing.T) {
		config, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput().Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
			ModuleVersion: "1.0.0",
		})
		require.NoError(t, err)

		configYAML, err := yaml.Marshal(config)
		require.NoError(t, err, "failed to marshal config")

		goldenFilePath := filepath.Join("testdata", "config.yaml")
		goldenFile, err := os.ReadFile(goldenFilePath)
		require.NoError(t, err, "failed to load golden file")

		require.NoError(t, err)
		require.Equal(t, string(goldenFile), string(configYAML))
	})

	t.Run("failed to make otlp exporter config", func(t *testing.T) {
		_, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test-fail").WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("nonexistent-secret", "default", "user", "password")).Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to make otlp exporter config")
	})

	t.Run("otlp input disabled", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput().WithOTLPInput(false).Build()}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Service.Pipelines, "logs/test")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test"].Receivers, "otlp")

		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[1], "transform/set-observed-time-if-zero")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[2], "istio_noise_filter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[3], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[4], "filter/drop-if-input-source-otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[5], "resource/insert-cluster-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[6], "service_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[7], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[8], "istio_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test"].Processors[9], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines["logs/test"].Exporters, "otlp/test")
	})

	t.Run("otlp input disabled multi pipeline", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			// should configure filter/drop-if-input-source-otlp filter
			testutils.NewLogPipelineBuilder().WithName("test-otlp-disabled").WithOTLPOutput().WithOTLPInput(false).Build(),
			// should not configure filter/drop-if-input-source-otlp filter
			testutils.NewLogPipelineBuilder().WithName("test-otlp-enabled").WithOTLPOutput().WithOTLPInput(true).Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Service.Pipelines, "logs/test-otlp-disabled")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Receivers, "otlp")

		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[1], "transform/set-observed-time-if-zero")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[2], "istio_noise_filter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[3], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[4], "filter/drop-if-input-source-otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[5], "resource/insert-cluster-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[6], "service_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[7], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[8], "istio_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Processors[9], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-otlp-disabled"].Exporters, "otlp/test-otlp-disabled")

		require.Contains(t, collectorConfig.Service.Pipelines, "logs/test-otlp-enabled")
		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Receivers, "otlp")

		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[1], "transform/set-observed-time-if-zero")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[2], "istio_noise_filter")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[3], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[4], "resource/insert-cluster-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[5], "service_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[6], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[7], "istio_enrichment")
		require.Equal(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Processors[8], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines["logs/test-otlp-enabled"].Exporters, "otlp/test-otlp-enabled")
	})
}
