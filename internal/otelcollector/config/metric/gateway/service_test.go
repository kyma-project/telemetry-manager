package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestService(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("single pipeline topology", func(t *testing.T) {
		t.Run("with no inputs enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPInput(false).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-if-input-source-otlp",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with prometheus input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithPrometheusInput(true).WithPrometheusInputDiagnosticMetrics(true).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with prometheus input enabled and diagnostic metrics disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithPrometheusInput(true).WithPrometheusInputDiagnosticMetrics(false).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"filter/drop-diagnostic-metrics-if-input-source-prometheus",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with prometheus input enabled and diagnostic metrics implicitly disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithPrometheusInput(true).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"filter/drop-diagnostic-metrics-if-input-source-prometheus",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
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
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"transform/set-instrumentation-scope-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-k8s-cluster-metrics",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
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
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"transform/set-instrumentation-scope-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-runtime-container-metrics",
				"filter/drop-k8s-cluster-metrics",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
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
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"transform/set-instrumentation-scope-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-runtime-pod-metrics",
				"filter/drop-k8s-cluster-metrics",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with istio input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithIstioInput(true).WithIstioInputDiagnosticMetrics(true).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with istio input enabled and diagnostic metrics disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithIstioInput(true).WithIstioInputDiagnosticMetrics(false).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-diagnostic-metrics-if-input-source-istio",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with istio input enabled and diagnostic metrics implicitly disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithIstioInput(true).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-diagnostic-metrics-if-input-source-istio",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with otlp input implicitly enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
		})

		t.Run("with otlp input explicitly enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(
				ctx,
				[]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithName("test").WithOTLPInput(true).Build(),
				},
				BuildOptions{},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-input")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-attributes-enrichment")
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-output")

			require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-input"].Receivers)
			require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-input"].Processors)
			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-input"].Exporters)

			require.Equal(t, []string{"routing/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Receivers)
			require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Processors)
			require.Equal(t, []string{"forward/test"}, collectorConfig.Service.Pipelines["metrics/test-attributes-enrichment"].Exporters)

			require.Equal(t, []string{"routing/test", "forward/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Receivers)
			require.Equal(t, []string{
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"transform/set-instrumentation-scope-kyma",
				"resource/insert-cluster-name",
				"resource/delete-skip-enrichment-attribute",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test-output"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["metrics/test-output"].Exporters)
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
			BuildOptions{},
		)
		require.NoError(t, err)

		// Test service configuration for MetricPipeline "test-1"
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-1-input")
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-1-attributes-enrichment")
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-1-output")

		require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/test-1-input"].Receivers)
		require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-1-input"].Processors)
		require.Equal(t, []string{"routing/test-1"}, collectorConfig.Service.Pipelines["metrics/test-1-input"].Exporters)

		require.Equal(t, []string{"routing/test-1"}, collectorConfig.Service.Pipelines["metrics/test-1-attributes-enrichment"].Receivers)
		require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-1-attributes-enrichment"].Processors)
		require.Equal(t, []string{"forward/test-1"}, collectorConfig.Service.Pipelines["metrics/test-1-attributes-enrichment"].Exporters)

		require.Equal(t, []string{"routing/test-1", "forward/test-1"}, collectorConfig.Service.Pipelines["metrics/test-1-output"].Receivers)
		require.Equal(t, []string{
			"transform/set-instrumentation-scope-runtime",
			"filter/drop-if-input-source-prometheus",
			"filter/drop-if-input-source-istio",
			"filter/test-1-filter-by-namespace-runtime-input",
			"filter/drop-k8s-cluster-metrics",
			"transform/set-instrumentation-scope-kyma",
			"resource/insert-cluster-name",
			"resource/delete-skip-enrichment-attribute",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-1-output"].Processors)
		require.Equal(t, []string{"otlp/test-1"}, collectorConfig.Service.Pipelines["metrics/test-1-output"].Exporters)

		// Test service configuration for MetricPipeline "test-2"
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-2-input")
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-2-attributes-enrichment")
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-2-output")

		require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-2-input"].Receivers)
		require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-2-input"].Processors)
		require.Equal(t, []string{"routing/test-2"}, collectorConfig.Service.Pipelines["metrics/test-2-input"].Exporters)

		require.Equal(t, []string{"routing/test-2"}, collectorConfig.Service.Pipelines["metrics/test-2-attributes-enrichment"].Receivers)
		require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-2-attributes-enrichment"].Processors)
		require.Equal(t, []string{"forward/test-2"}, collectorConfig.Service.Pipelines["metrics/test-2-attributes-enrichment"].Exporters)

		require.Equal(t, []string{"routing/test-2", "forward/test-2"}, collectorConfig.Service.Pipelines["metrics/test-2-output"].Receivers)
		require.Equal(t, []string{
			"filter/drop-if-input-source-runtime",
			"filter/drop-if-input-source-istio",
			"filter/test-2-filter-by-namespace-prometheus-input",
			"filter/drop-diagnostic-metrics-if-input-source-prometheus",
			"transform/set-instrumentation-scope-kyma",
			"resource/insert-cluster-name",
			"resource/delete-skip-enrichment-attribute",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-2-output"].Processors)
		require.Equal(t, []string{"otlp/test-2"}, collectorConfig.Service.Pipelines["metrics/test-2-output"].Exporters)

		// Test service configuration for MetricPipeline "test-3"
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-3-input")
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-3-attributes-enrichment")
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-3-output")

		require.Equal(t, []string{"otlp", "singleton_receiver_creator/kymastats"}, collectorConfig.Service.Pipelines["metrics/test-3-input"].Receivers)
		require.Equal(t, []string{"memory_limiter"}, collectorConfig.Service.Pipelines["metrics/test-3-input"].Processors)
		require.Equal(t, []string{"routing/test-3"}, collectorConfig.Service.Pipelines["metrics/test-3-input"].Exporters)

		require.Equal(t, []string{"routing/test-3"}, collectorConfig.Service.Pipelines["metrics/test-3-attributes-enrichment"].Receivers)
		require.Equal(t, []string{"k8sattributes", "transform/resolve-service-name"}, collectorConfig.Service.Pipelines["metrics/test-3-attributes-enrichment"].Processors)
		require.Equal(t, []string{"forward/test-3"}, collectorConfig.Service.Pipelines["metrics/test-3-attributes-enrichment"].Exporters)

		require.Equal(t, []string{"routing/test-3", "forward/test-3"}, collectorConfig.Service.Pipelines["metrics/test-3-output"].Receivers)
		require.Equal(t, []string{
			"filter/drop-if-input-source-runtime",
			"filter/drop-if-input-source-prometheus",
			"filter/drop-diagnostic-metrics-if-input-source-istio",
			"transform/set-instrumentation-scope-kyma",
			"resource/insert-cluster-name",
			"resource/delete-skip-enrichment-attribute",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-3-output"].Processors)
		require.Equal(t, []string{"otlp/test-3"}, collectorConfig.Service.Pipelines["metrics/test-3-output"].Exporters)

		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_1")
		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_2")
		require.Contains(t, envVars, "OTLP_ENDPOINT_TEST_3")
	})
}
