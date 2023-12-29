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

	t.Run("insert cluster name processor", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, 1, len(collectorConfig.Processors.InsertClusterName.Attributes))
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterName.Attributes[0].Action)
		require.Equal(t, "k8s.cluster.name", collectorConfig.Processors.InsertClusterName.Attributes[0].Key)
		require.Equal(t, "${KUBERNETES_SERVICE_HOST}", collectorConfig.Processors.InsertClusterName.Attributes[0].Value)
	})

	t.Run("memory limit processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, "0.1s", collectorConfig.Processors.MemoryLimiter.CheckInterval)
		require.Equal(t, 75, collectorConfig.Processors.MemoryLimiter.LimitPercentage)
		require.Equal(t, 20, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage)
	})

	t.Run("batch processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchSize)
		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchMaxSize)
		require.Equal(t, "10s", collectorConfig.Processors.Batch.Timeout)
	})

	t.Run("k8s attributes processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
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
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().OtlpInput(false).Build()})
		require.NoError(t, err)

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceRuntime)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceRuntime.Metrics.DataPoint, 1)
		require.Equal(t, "resource.attributes[\"kyma.source\"] == \"runtime\"", collectorConfig.Processors.DropIfInputSourceRuntime.Metrics.DataPoint[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourcePrometheus)
		require.Len(t, collectorConfig.Processors.DropIfInputSourcePrometheus.Metrics.DataPoint, 1)
		require.Equal(t, "resource.attributes[\"kyma.source\"] == \"prometheus\"", collectorConfig.Processors.DropIfInputSourcePrometheus.Metrics.DataPoint[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceIstio)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceIstio.Metrics.DataPoint, 1)
		require.Equal(t, "resource.attributes[\"kyma.source\"] == \"istio\"", collectorConfig.Processors.DropIfInputSourceIstio.Metrics.DataPoint[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceOtlp)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceOtlp.Metrics.Metric, 1)
		require.Equal(t, "resource.attributes[\"kyma.source\"] == nil", collectorConfig.Processors.DropIfInputSourceOtlp.Metrics.Metric[0])
	})

	t.Run("namespace filter processor using include", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test").
				RuntimeInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
				PrometheusInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
				IstioInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
				OtlpInput(true, testutils.IncludeNamespaces("ns-1", "ns-2")).
				Build()},
		)
		require.NoError(t, err)

		namespaceFilters := collectorConfig.Processors.NamespaceFilters
		require.NotNil(t, namespaceFilters)

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-runtime-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric, 1)
		expectedCondition := "resource.attributes[\"kyma.source\"] == \"runtime\" and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-prometheus-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric, 1)
		expectedCondition = "resource.attributes[\"kyma.source\"] == \"prometheus\" and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-istio-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric, 1)
		expectedCondition = "resource.attributes[\"kyma.source\"] == \"istio\" and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-otlp-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric, 1)
		expectedCondition = "resource.attributes[\"kyma.source\"] == nil and not((resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\"))"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric[0])
	})

	t.Run("namespace filter processor using exclude", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test").
				RuntimeInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
				PrometheusInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
				IstioInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
				OtlpInput(true, testutils.ExcludeNamespaces("ns-1", "ns-2")).
				Build()},
		)
		require.NoError(t, err)

		namespaceFilters := collectorConfig.Processors.NamespaceFilters
		require.NotNil(t, namespaceFilters)

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-runtime-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric, 1)
		expectedCondition := "resource.attributes[\"kyma.source\"] == \"runtime\" and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-runtime-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-prometheus-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric, 1)
		expectedCondition = "resource.attributes[\"kyma.source\"] == \"prometheus\" and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-prometheus-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-istio-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric, 1)
		expectedCondition = "resource.attributes[\"kyma.source\"] == \"istio\" and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-istio-input"].Metrics.Metric[0])

		require.Contains(t, namespaceFilters, "filter/test-filter-by-namespace-otlp-input")
		require.Len(t, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric, 1)
		expectedCondition = "resource.attributes[\"kyma.source\"] == nil and (resource.attributes[\"k8s.namespace.name\"] == \"ns-1\" or resource.attributes[\"k8s.namespace.name\"] == \"ns-2\")"
		require.Equal(t, expectedCondition, namespaceFilters["filter/test-filter-by-namespace-otlp-input"].Metrics.Metric[0])
	})

	t.Run("diagnostic metric filter processor prometheus input using exclude", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test").
				PrometheusInput(true).
				PrometheusInputDiagnosticMetrics(false).
				Build()},
		)
		require.NoError(t, err)

		prometheusScrapeFilter := collectorConfig.Processors.DropDiagnosticMetricsIfInputSourcePrometheus
		require.NotNil(t, prometheusScrapeFilter)
		require.Nil(t, collectorConfig.Processors.DropDiagnosticMetricsIfInputSourceIstio)
		expectedCondition := "resource.attributes[\"kyma.source\"] == \"prometheus\" and (name == \"up\" or name == \"scrape_duration_seconds\" or name == \"scrape_samples_scraped\" or name == \"scrape_samples_post_metric_relabeling\" or name == \"scrape_series_added\")"
		require.Len(t, prometheusScrapeFilter.Metrics.Metric, 1)
		require.Equal(t, prometheusScrapeFilter.Metrics.Metric[0], expectedCondition)
	})

	t.Run("diagnostic metric filter processor prometheus input using exclude", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test").
				IstioInput(true).
				IstioInputDiagnosticMetrics(false).
				Build()},
		)
		require.NoError(t, err)

		istioScrapeFilter := collectorConfig.Processors.DropDiagnosticMetricsIfInputSourceIstio
		require.NotNil(t, istioScrapeFilter)

		require.Nil(t, collectorConfig.Processors.DropDiagnosticMetricsIfInputSourcePrometheus)

		require.Len(t, istioScrapeFilter.Metrics.Metric, 1)
		expectedCondition := "resource.attributes[\"kyma.source\"] == \"istio\" and (name == \"up\" or name == \"scrape_duration_seconds\" or name == \"scrape_samples_scraped\" or name == \"scrape_samples_post_metric_relabeling\" or name == \"scrape_series_added\")"
		require.Equal(t, istioScrapeFilter.Metrics.Metric[0], expectedCondition)
	})
}
