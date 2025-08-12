package loggateway

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestProcessors(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("insert cluster attributes processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "test-cluster",
			ClusterUID:    "test-cluster-uid",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Equal(t, 3, len(collectorConfig.Processors.InsertClusterAttributes.Attributes))
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterAttributes.Attributes[0].Action)
		require.Equal(t, "k8s.cluster.name", collectorConfig.Processors.InsertClusterAttributes.Attributes[0].Key)
		require.Equal(t, "test-cluster", collectorConfig.Processors.InsertClusterAttributes.Attributes[0].Value)
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterAttributes.Attributes[1].Action)
		require.Equal(t, "k8s.cluster.uid", collectorConfig.Processors.InsertClusterAttributes.Attributes[1].Key)
		require.Equal(t, "test-cluster-uid", collectorConfig.Processors.InsertClusterAttributes.Attributes[1].Value)
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterAttributes.Attributes[2].Action)
		require.Equal(t, "cloud.provider", collectorConfig.Processors.InsertClusterAttributes.Attributes[2].Key)
		require.Equal(t, "test-cloud-provider", collectorConfig.Processors.InsertClusterAttributes.Attributes[2].Value)
	})

	t.Run("memory limit processors", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "test-cluster",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Equal(t, "1s", collectorConfig.Processors.MemoryLimiter.CheckInterval)
		require.Equal(t, 75, collectorConfig.Processors.MemoryLimiter.LimitPercentage)
		require.Equal(t, 15, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage)
	})

	t.Run("batch processors", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "test-cluster",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Equal(t, 512, collectorConfig.Processors.Batch.SendBatchSize)
		require.Equal(t, 512, collectorConfig.Processors.Batch.SendBatchMaxSize)
		require.Equal(t, "10s", collectorConfig.Processors.Batch.Timeout)
	})

	t.Run("k8s attributes processors", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "test-cluster",
			CloudProvider: "test-cloud-provider",
		})
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

	t.Run("set observed time when not present", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()}, BuildOptions{
			ClusterName:   "test-cluster",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Len(t, collectorConfig.Processors.SetObsTimeIfZero.LogStatements[0].Conditions, 1)
		require.Equal(t, "log.observed_time_unix_nano == 0", collectorConfig.Processors.SetObsTimeIfZero.LogStatements[0].Conditions[0])

		require.Len(t, collectorConfig.Processors.SetObsTimeIfZero.LogStatements[0].Statements, 1)
		require.Equal(t, "set(log.observed_time, Now())", collectorConfig.Processors.SetObsTimeIfZero.LogStatements[0].Statements[0])
	})

	t.Run("include namespaces", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithName("dummy").
			WithOTLPInput(true, testutils.IncludeNamespaces("kyma-system", "default")).
			WithOTLPOutput().
			Build()

		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{pipeline}, BuildOptions{})
		require.NoError(t, err)

		namespaceFilters := collectorConfig.Processors.Dynamic
		require.NotNil(t, namespaceFilters)

		expectedFilterID := "filter/dummy-filter-by-namespace"
		require.Contains(t, namespaceFilters, expectedFilterID)
		require.IsType(t, &FilterProcessor{}, namespaceFilters[expectedFilterID])

		filterProcessor := namespaceFilters[expectedFilterID].(*FilterProcessor)
		actualStatements := filterProcessor.Logs.Log
		require.Len(t, actualStatements, 1)

		expectedStatement := `resource.attributes["k8s.namespace.name"] != nil and not(resource.attributes["k8s.namespace.name"] == "kyma-system" or resource.attributes["k8s.namespace.name"] == "default")`
		require.Equal(t, expectedStatement, actualStatements[0])
	})

	t.Run("exclude namespaces", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithName("dummy").
			WithOTLPInput(true, testutils.ExcludeNamespaces("kyma-system", "default")).
			WithOTLPOutput().
			Build()

		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{pipeline}, BuildOptions{})
		require.NoError(t, err)

		namespaceFilters := collectorConfig.Processors.Dynamic
		require.NotNil(t, namespaceFilters)

		expectedFilterID := "filter/dummy-filter-by-namespace"
		require.Contains(t, namespaceFilters, expectedFilterID)
		require.IsType(t, &FilterProcessor{}, namespaceFilters[expectedFilterID])

		filterProcessor := namespaceFilters[expectedFilterID].(*FilterProcessor)
		actualStatements := filterProcessor.Logs.Log
		require.Len(t, actualStatements, 1)

		expectedStatement := `(resource.attributes["k8s.namespace.name"] == "kyma-system" or resource.attributes["k8s.namespace.name"] == "default")`
		require.Equal(t, expectedStatement, actualStatements[0])
	})

	t.Run("OTLP input disabled", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithName("dummy").
			WithOTLPInput(false).
			WithOTLPOutput().
			Build()

		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{pipeline}, BuildOptions{})
		require.NoError(t, err)

		dropOTLPInputFilter := collectorConfig.Processors.DropIfInputSourceOTLP
		require.NotNil(t, dropOTLPInputFilter)

		actualStatements := dropOTLPInputFilter.Logs.Log
		require.Len(t, actualStatements, 1)

		expectedStatement := `(log.observed_time != nil or log.time != nil)`
		require.Equal(t, expectedStatement, actualStatements[0])
	})
}
