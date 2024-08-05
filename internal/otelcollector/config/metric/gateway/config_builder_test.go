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
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMakeConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}
	gatewayNamespace := "test-namespace"

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
			},
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
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
			gatewayNamespace,
			false,
		)
		require.NoError(t, err)

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "json", collectorConfig.Service.Telemetry.Logs.Encoding)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").Build(),
			},
			gatewayNamespace,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, 256, collectorConfig.Exporters["otlp/test"].OTLP.SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
				testutils.NewMetricPipelineBuilder().WithName("test-3").Build(),
			},
			gatewayNamespace,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-1"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-2"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-3"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		t.Run("with no inputs enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPInput(false).Build(),
				},
				gatewayNamespace,
				true,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.NotContains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "singleton_receiver_creator/kymastats")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-if-input-source-otlp",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with prometheus input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithPrometheusInput(true).WithPrometheusInputDiagnosticMetrics(true).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with prometheus input enabled and diagnostic metrics disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithPrometheusInput(true).WithPrometheusInputDiagnosticMetrics(false).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"filter/drop-diagnostic-metrics-if-input-source-prometheus",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with prometheus input enabled and diagnostic metrics implicitly disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithPrometheusInput(true).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"filter/drop-diagnostic-metrics-if-input-source-prometheus",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with runtime input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					// Simulate the default scenario for runtime input by enabling both pod and container metrics
					// NOTE: the pod and container metrics are enabled by default on the CRD level when the runtime input is defined
					testutils.NewMetricPipelineBuilder().
						WithName("test").
						WithRuntimeInput(true).
						WithRuntimeInputPodMetrics(true).
						WithRuntimeInputContainerMetrics(true).
						Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with runtime input enabled and only pod metrics enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("test").
						WithRuntimeInput(true).
						WithRuntimeInputContainerMetrics(false).
						Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-runtime-container-metrics",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with runtime input enabled and only container metrics enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().
						WithName("test").
						WithRuntimeInput(true).
						WithRuntimeInputPodMetrics(false).
						Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-runtime-pod-metrics",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with istio input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithIstioInput(true).WithIstioInputDiagnosticMetrics(true).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with istio input enabled and diagnostic metrics disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithIstioInput(true).WithIstioInputDiagnosticMetrics(false).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-diagnostic-metrics-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with istio input enabled and diagnostic metrics implicitly disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithIstioInput(true).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-diagnostic-metrics-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with otlp input implicitly enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with otlp input explicitly enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPInput(true).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with kyma input annotation existing and kyma input is allowed", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithAnnotations(map[string]string{"experimental-kyma-input": "true"}).Build(),
				},
				gatewayNamespace,
				true,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "singleton_receiver_creator/kymastats")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with kyma input annotation existing and kyma input is not allowed", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithAnnotations(map[string]string{"experimental-kyma-input": "true"}).Build(),
				},
				gatewayNamespace,
				false,
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.NotContains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "singleton_receiver_creator/kymastats")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				// Simulate the default scenario for runtime input by enabling both pod and container metrics
				// NOTE: the pod and container metrics are enabled by default on the CRD level when the runtime input is defined
				testutils.NewMetricPipelineBuilder().
					WithName("test-1").
					WithRuntimeInput(true, testutils.ExcludeNamespaces(namespaces.System()...)).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputContainerMetrics(true).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test-2").
					WithPrometheusInput(true, testutils.ExcludeNamespaces(namespaces.System()...)).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test-3").
					WithIstioInput(true).
					Build(),
			},
			gatewayNamespace,
			false,
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-3")

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Receivers, "otlp")
		require.Equal(t, []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-if-input-source-prometheus",
			"filter/drop-if-input-source-istio",
			"filter/test-1-filter-by-namespace-runtime-input",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-1"].Processors)

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Receivers, "otlp")
		require.Equal(t, []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-if-input-source-runtime",
			"filter/drop-if-input-source-istio",
			"filter/test-2-filter-by-namespace-prometheus-input",
			"filter/drop-diagnostic-metrics-if-input-source-prometheus",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-2"].Processors)

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-3")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-3"].Exporters, "otlp/test-3")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-3"].Receivers, "otlp")
		require.Equal(t, []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-if-input-source-runtime",
			"filter/drop-if-input-source-prometheus",
			"filter/drop-diagnostic-metrics-if-input-source-istio",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-3"].Processors)

		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_1")
		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_2")
		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_3")
	})

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name           string
			goldenFileName string
			withOtlpInput  bool
		}{
			{
				name:           "OTLP Endpoint enabled",
				goldenFileName: "config.yaml",
				withOtlpInput:  true,
			},
			{
				name:           "OTLP Endpoint disabled",
				goldenFileName: "config_otlp_disabled.yaml",
				withOtlpInput:  false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				config, _, err := sut.Build(
					context.Background(),
					[]telemetryv1alpha1.MetricPipeline{
						testutils.NewMetricPipelineBuilder().
							WithName("test").
							WithOTLPInput(tt.withOtlpInput).
							WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
					},
					gatewayNamespace,
					false,
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
