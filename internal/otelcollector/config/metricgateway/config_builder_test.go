package metricgateway

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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeConfig(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
			},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)

		expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")

		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, actualExporterConfig.(*common.OTLPExporter).Endpoint)

		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST")
		require.Equal(t, "http://localhost", string(envVars["OTLP_ENDPOINT_TEST"]))
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, actualExporterConfig.(*common.OTLPExporter).TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-insecure").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
			},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")

		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.(*common.OTLPExporter).TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-basic-auth").WithOTLPOutput(testutils.OTLPBasicAuth("user", "password")).Build(),
			},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")

		actualExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := actualExporterConfig.(*common.OTLPExporter).Headers
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
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-custom-header")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test-custom-header"]
		headers := otlpExporterConfig.(*common.OTLPExporter).Headers
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
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-mtls")

		otlpExporterConfig := collectorConfig.Exporters["otlp/test-mtls"]
		require.Equal(t, "${OTLP_TLS_CERT_PEM_TEST_MTLS}", otlpExporterConfig.(*common.OTLPExporter).TLS.CertPem)
		require.Equal(t, "${OTLP_TLS_KEY_PEM_TEST_MTLS}", otlpExporterConfig.(*common.OTLPExporter).TLS.KeyPem)

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
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
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
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)

		metricreaders := []common.MetricReader{
			{
				Pull: common.PullMetricReader{
					Exporter: common.MetricExporter{
						Prometheus: common.PrometheusMetricExporter{
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
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)
		require.Equal(t, maxQueueSize, collectorConfig.Exporters["otlp/test"].(*common.OTLPExporter).SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-3").Build(),
			},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)

		require.NoError(t, err)

		expectedQueueSize := 85 // Total queue size (256) divided by the number of pipelines (3)
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-1"].(*common.OTLPExporter).SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-2"].(*common.OTLPExporter).SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, expectedQueueSize, collectorConfig.Exporters["otlp/test-3"].(*common.OTLPExporter).SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("exporters names", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-3").Build(),
			},
			BuildOptions{
				ClusterName:   "${KUBERNETES_SERVICE_HOST}",
				CloudProvider: "test-cloud-provider",
			},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-3")
	})

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name                string
			pipelines           []telemetryv1alpha1.MetricPipeline
			goldenFileName      string
			overwriteGoldenFile bool
		}{
			{
				name:           "simple single pipeline setup",
				goldenFileName: "setup-simple.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("test").
						WithOTLPInput(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "complex pipeline with comprehensive configuration",
				goldenFileName: "setup-comprehensive.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithRuntimeInput(true, testutils.IncludeNamespaces("production", "staging")).
						WithRuntimeInputPodMetrics(true).
						WithRuntimeInputContainerMetrics(true).
						WithRuntimeInputNodeMetrics(true).
						WithPrometheusInput(true, testutils.ExcludeNamespaces("kube-system")).
						WithPrometheusInputDiagnosticMetrics(true).
						WithIstioInput(true).
						WithIstioInputEnvoyMetrics(true).
						WithOTLPInput(true, testutils.IncludeNamespaces("apps")).
						WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com")).
						WithTransform(telemetryv1alpha1.TransformSpec{
							Conditions: []string{"resource.attributes[\"k8s.namespace.name\"] == \"production\""},
							Statements: []string{"set(attributes[\"environment\"], \"prod\")"},
						}).Build(),
				},
			},
			{
				name:           "pipeline with OTLP input disabled",
				goldenFileName: "otlp-disabled.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("test").
						WithOTLPInput(false).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with runtime input and namespace filters",
				goldenFileName: "runtime-namespace-filters.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithRuntimeInput(true,
							testutils.IncludeNamespaces("monitoring", "observability"),
							testutils.ExcludeNamespaces("kube-system", "istio-system"),
						).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with prometheus input and namespace filters",
				goldenFileName: "prometheus-namespace-filters.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithPrometheusInput(true,
							testutils.IncludeNamespaces("monitoring", "observability"),
							testutils.ExcludeNamespaces("kube-system", "istio-system"),
						).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with istio input and namespace filters",
				goldenFileName: "istio-namespace-filters.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithIstioInput(true,
							testutils.IncludeNamespaces("monitoring", "observability"),
							testutils.ExcludeNamespaces("kube-system", "istio-system"),
						).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with OTLP input and namespace filters",
				goldenFileName: "otlp-namespace-filters.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithOTLPInput(true,
							testutils.IncludeNamespaces("monitoring", "observability"),
							testutils.ExcludeNamespaces("kube-system", "istio-system"),
						).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with multiple input types and mixed configurations",
				goldenFileName: "multiple-inputs-mixed.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithRuntimeInput(true, testutils.IncludeNamespaces("default")).
						WithPrometheusInput(true, testutils.ExcludeNamespaces("kube-system")).
						WithIstioInput(false).
						WithOTLPInput(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with all runtime input resources desiabled",
				goldenFileName: "runtime-resources-all-disabled.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithRuntimeInput(true).
						WithRuntimeInputPodMetrics(false).
						WithRuntimeInputContainerMetrics(false).
						WithRuntimeInputNodeMetrics(false).
						WithRuntimeInputVolumeMetrics(false).
						WithRuntimeInputDeploymentMetrics(false).
						WithRuntimeInputDaemonSetMetrics(false).
						WithRuntimeInputStatefulSetMetrics(false).
						WithRuntimeInputJobMetrics(false).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with some runtime input resources disabled",
				goldenFileName: "runtime-resources-some-disabled.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithRuntimeInput(true).
						WithRuntimeInputPodMetrics(true).
						WithRuntimeInputContainerMetrics(false).
						WithRuntimeInputNodeMetrics(false).
						WithRuntimeInputVolumeMetrics(true).
						WithRuntimeInputDeploymentMetrics(true).
						WithRuntimeInputDaemonSetMetrics(true).
						WithRuntimeInputStatefulSetMetrics(true).
						WithRuntimeInputJobMetrics(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with prometheus diagnostic metrics",
				goldenFileName: "prometheus-diagnostic.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithPrometheusInput(true).
						WithPrometheusInputDiagnosticMetrics(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with istio envoy metrics",
				goldenFileName: "istio-envoy.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithIstioInput(true).
						WithIstioInputEnvoyMetrics(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with istio diagnostic metrics",
				goldenFileName: "istio-diagnostic.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithIstioInput(true).
						WithIstioInputDiagnosticMetrics(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "pipeline with all inputs disabled except OTLP",
				goldenFileName: "otlp-only.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithRuntimeInput(false).
						WithPrometheusInput(false).
						WithIstioInput(false).
						WithOTLPInput(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
				},
			},
			{
				name:           "two pipelines with user-defined transforms",
				goldenFileName: "user-defined-transforms.yaml",
				pipelines: []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("cls").
						WithOTLPInput(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
						WithTransform(telemetryv1alpha1.TransformSpec{
							Conditions: []string{"IsMatch(body, \".*error.*\")"},
							Statements: []string{
								"set(attributes[\"log.level\"], \"error\")",
								"set(body, \"transformed1\")",
							},
						}).Build(),
					testutils.NewMetricPipelineBuilder().
						WithName("dynatrace").
						WithOTLPInput(true).
						WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
						WithTransform(telemetryv1alpha1.TransformSpec{
							Conditions: []string{"IsMatch(body, \".*error.*\")"},
							Statements: []string{
								"set(attributes[\"log.level\"], \"error\")",
								"set(body, \"transformed2\")",
							},
						}).Build(),
				},
			},
		}

		buildOptions := BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config, _, err := sut.Build(t.Context(), tt.pipelines, buildOptions)
				require.NoError(t, err)

				configYAML, err := yaml.Marshal(config)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				if tt.overwriteGoldenFile {
					err = os.WriteFile(goldenFilePath, configYAML, 0600)
					require.NoError(t, err, "failed to overwrite golden file")

					t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", tt.goldenFileName)
				}

				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.NoError(t, err)
				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
