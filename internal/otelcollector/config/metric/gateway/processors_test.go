package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestProcessors(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("insert cluster name processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Equal(t, 1, len(collectorConfig.Processors.InsertClusterName.Attributes))
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterName.Attributes[0].Action)
		require.Equal(t, "k8s.cluster.name", collectorConfig.Processors.InsertClusterName.Attributes[0].Key)
		require.Equal(t, "${KUBERNETES_SERVICE_HOST}", collectorConfig.Processors.InsertClusterName.Attributes[0].Value)
	})

	t.Run("memory limit processors", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Equal(t, "1s", collectorConfig.Processors.MemoryLimiter.CheckInterval)
		require.Equal(t, 75, collectorConfig.Processors.MemoryLimiter.LimitPercentage)
		require.Equal(t, 15, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage)
	})

	t.Run("batch processors", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchSize)
		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchMaxSize)
		require.Equal(t, "10s", collectorConfig.Processors.Batch.Timeout)
	})

	t.Run("k8s attributes processors", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Equal(t, "serviceAccount", collectorConfig.Processors.K8sAttributes.AuthType)
		require.False(t, collectorConfig.Processors.K8sAttributes.Passthrough)

		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.pod.name")

		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.node.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.namespace.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.deployment.name")

		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.statefulset.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.daemonset.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.cronjob.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.job.name")

		require.Equal(t, 3, len(collectorConfig.Processors.K8sAttributes.PodAssociation))
		require.Equal(t, "resource_attribute", collectorConfig.Processors.K8sAttributes.PodAssociation[0].Sources[0].From)
		require.Equal(t, "k8s.pod.ip", collectorConfig.Processors.K8sAttributes.PodAssociation[0].Sources[0].Name)

		require.Equal(t, "resource_attribute", collectorConfig.Processors.K8sAttributes.PodAssociation[1].Sources[0].From)
		require.Equal(t, "k8s.pod.uid", collectorConfig.Processors.K8sAttributes.PodAssociation[1].Sources[0].Name)

		require.Equal(t, "connection", collectorConfig.Processors.K8sAttributes.PodAssociation[2].Sources[0].From)
	})

	t.Run("drop by input source filter", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithOTLPInput(false).Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceRuntime)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceRuntime.Metrics.Metric, 1)
		require.Equal(t, "instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\"", collectorConfig.Processors.DropIfInputSourceRuntime.Metrics.Metric[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourcePrometheus)
		require.Len(t, collectorConfig.Processors.DropIfInputSourcePrometheus.Metrics.Metric, 1)
		require.Equal(t, "instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\"", collectorConfig.Processors.DropIfInputSourcePrometheus.Metrics.Metric[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceIstio)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceIstio.Metrics.Metric, 1)
		require.Equal(t, "instrumentation_scope.name == \"io.kyma-project.telemetry/istio\"", collectorConfig.Processors.DropIfInputSourceIstio.Metrics.Metric[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceOtlp)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceOtlp.Metrics.Metric, 1)
		require.Equal(t,
			"not(instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" or "+
				"instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" or "+
				"instrumentation_scope.name == \"io.kyma-project.telemetry/istio\")",
			collectorConfig.Processors.DropIfInputSourceOtlp.Metrics.Metric[0],
		)
	})

	t.Run("namespace filter processor using include", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithRuntimeInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
					WithPrometheusInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
					WithIstioInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
					WithOTLPInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
					Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		namespaceFilters := collectorConfig.Processors.NamespaceFilters
		require.NotNil(t, namespaceFilters)

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-runtime-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric, 1)
		expectedCondition := "instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-prometheus-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric, 1)
		expectedCondition = "instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-istio-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric, 1)
		expectedCondition = "instrumentation_scope.name == \"io.kyma-project.telemetry/istio\" and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-otlp-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric, 1)
		expectedCondition = "not(instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" or " +
			"instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" or " +
			"instrumentation_scope.name == \"io.kyma-project.telemetry/istio\") and " +
			"not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric[0])
	})

	t.Run("namespace filter processor using exclude", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithRuntimeInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
					WithPrometheusInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
					WithIstioInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
					WithOTLPInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
					Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		namespaceFilters := collectorConfig.Processors.NamespaceFilters
		require.NotNil(t, namespaceFilters)

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-runtime-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric, 1)
		expectedCondition := "instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-prometheus-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric, 1)
		expectedCondition = "instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-istio-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric, 1)
		expectedCondition = "instrumentation_scope.name == \"io.kyma-project.telemetry/istio\" and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-otlp-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric, 1)
		expectedCondition = "not(instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" or " +
			"instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" or " +
			"instrumentation_scope.name == \"io.kyma-project.telemetry/istio\") and " +
			"(resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric[0])
	})

	t.Run("prometheus diagnostic metrics filter processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithPrometheusInput(true).
					WithPrometheusInputDiagnosticMetrics(false).
					Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		prometheusScrapeFilter := collectorConfig.Processors.DropDiagnosticMetricsIfInputSourcePrometheus
		require.NotNil(t, prometheusScrapeFilter)
		require.Nil(t, collectorConfig.Processors.DropDiagnosticMetricsIfInputSourceIstio)
		expectedCondition := "instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" and (name == \"up\" or name == \"scrape_duration_seconds\" or name == \"scrape_samples_scraped\" or name == \"scrape_samples_post_metric_relabeling\" or name == \"scrape_series_added\")"
		require.Len(t, prometheusScrapeFilter.Metrics.Metric, 1)
		require.Equal(t, expectedCondition, prometheusScrapeFilter.Metrics.Metric[0])
	})

	t.Run("istio diagnostic metrics filter processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithIstioInput(true).
					WithIstioInputDiagnosticMetrics(false).
					Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		istioScrapeFilter := collectorConfig.Processors.DropDiagnosticMetricsIfInputSourceIstio
		require.NotNil(t, istioScrapeFilter)

		require.Nil(t, collectorConfig.Processors.DropDiagnosticMetricsIfInputSourcePrometheus)

		require.Len(t, istioScrapeFilter.Metrics.Metric, 1)
		expectedCondition := "instrumentation_scope.name == \"io.kyma-project.telemetry/istio\" and (name == \"up\" or name == \"scrape_duration_seconds\" or name == \"scrape_samples_scraped\" or name == \"scrape_samples_post_metric_relabeling\" or name == \"scrape_series_added\")"
		require.Equal(t, expectedCondition, istioScrapeFilter.Metrics.Metric[0])
	})

	t.Run("runtime pod metrics filter processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		runtimePodMetricsFilter := collectorConfig.Processors.DropRuntimePodMetrics
		require.NotNil(t, runtimePodMetricsFilter)
		require.Len(t, runtimePodMetricsFilter.Metrics.Metric, 1)
		expectedCondition := "instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" and IsMatch(name, \"^k8s.pod.*\")"
		require.Equal(t, expectedCondition, runtimePodMetricsFilter.Metrics.Metric[0])
	})

	t.Run("runtime container metrics filter processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		runtimeContainerMetricsFilter := collectorConfig.Processors.DropRuntimeContainerMetrics
		require.NotNil(t, runtimeContainerMetricsFilter)
		require.Len(t, runtimeContainerMetricsFilter.Metrics.Metric, 1)
		expectedCondition := "instrumentation_scope.name == \"io.kyma-project.telemetry/runtime\" and IsMatch(name, \"(^k8s.container.*)|(^container.*)\")"
		require.Equal(t, expectedCondition, runtimeContainerMetricsFilter.Metrics.Metric[0])
	})

	t.Run("kyma instrumentation scope transform processor for kymastats receiver", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithAnnotations(map[string]string{KymaInputAnnotation: "true"}).Build(),
			},
			BuildOptions{
				InstrumentationScopeVersion: "main",
				KymaInputAllowed:            true,
			},
		)
		require.NoError(t, err)

		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeKyma)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopeKyma.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeKyma.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopeKyma.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeKyma.MetricStatements[0].Statements, 2)
		require.Equal(t, "set(version, \"main\") where name == \"otelcol/kymastats\"", collectorConfig.Processors.SetInstrumentationScopeKyma.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/kyma\") where name == \"otelcol/kymastats\"", collectorConfig.Processors.SetInstrumentationScopeKyma.MetricStatements[0].Statements[1])

	})

	t.Run("kyma instrumentation scope transform processor for k8sCluster receiver", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithRuntimeInput(true).Build(),
			},
			BuildOptions{
				InstrumentationScopeVersion: "main",
				K8sClusterReceiverAllowed:   true,
			},
		)
		require.NoError(t, err)

		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeK8sCluster)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopeK8sCluster.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeK8sCluster.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopeK8sCluster.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeK8sCluster.MetricStatements[0].Statements, 2)
		require.Equal(t, "set(version, \"main\") where name == \"otelcol/k8sclusterreceiver\"", collectorConfig.Processors.SetInstrumentationScopeK8sCluster.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/k8s_cluster\") where name == \"otelcol/k8sclusterreceiver\"", collectorConfig.Processors.SetInstrumentationScopeK8sCluster.MetricStatements[0].Statements[1])
	})
	t.Run("k8s cluster receiver filter metrics", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").
					WithRuntimeInput(true).
					Build(),
			},
			BuildOptions{K8sClusterReceiverAllowed: true},
		)
		require.NoError(t, err)

		runtimeContainerMetricsFilter := collectorConfig.Processors.DropK8sClusterMetrics
		require.NotNil(t, runtimeContainerMetricsFilter)
		require.Len(t, runtimeContainerMetricsFilter.Metrics.Metric, 1)
		require.Equal(t, k8sClusterMetricsDrop[0], runtimeContainerMetricsFilter.Metrics.Metric[0])
	})
}
