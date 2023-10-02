package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestProcessors(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	t.Run("resource processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, 1, len(collectorConfig.Processors.Resource.Attributes))
		require.Equal(t, "insert", collectorConfig.Processors.Resource.Attributes[0].Action)
		require.Equal(t, "k8s.cluster.name", collectorConfig.Processors.Resource.Attributes[0].Key)
		require.Equal(t, "${KUBERNETES_SERVICE_HOST}", collectorConfig.Processors.Resource.Attributes[0].Value)
	})

	t.Run("memory limit processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, "0.1s", collectorConfig.Processors.MemoryLimiter.CheckInterval)
		require.Equal(t, 75, collectorConfig.Processors.MemoryLimiter.LimitPercentage)
		require.Equal(t, 10, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage)
	})

	t.Run("batch processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchSize)
		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchMaxSize)
		require.Equal(t, "10s", collectorConfig.Processors.Batch.Timeout)
	})

	t.Run("k8s attributes processors", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
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
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourceRuntime)
		require.Len(t, collectorConfig.Processors.DropIfInputSourceRuntime.Metrics.DataPoint, 1)
		require.Equal(t, "resource.attributes[\"kyma.source\"] == \"runtime\"", collectorConfig.Processors.DropIfInputSourceRuntime.Metrics.DataPoint[0])

		require.NotNil(t, collectorConfig.Processors.DropIfInputSourcePrometheus)
		require.Len(t, collectorConfig.Processors.DropIfInputSourcePrometheus.Metrics.DataPoint, 1)
		require.Equal(t, "resource.attributes[\"kyma.source\"] == \"prometheus\"", collectorConfig.Processors.DropIfInputSourcePrometheus.Metrics.DataPoint[0])
	})
}
