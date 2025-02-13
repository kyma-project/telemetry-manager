package gateway

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
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		require.Equal(t, 2, len(collectorConfig.Processors.InsertClusterAttributes.Attributes))
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterAttributes.Attributes[0].Action)
		require.Equal(t, "k8s.cluster.name", collectorConfig.Processors.InsertClusterAttributes.Attributes[0].Key)
		require.Equal(t, "test-cluster", collectorConfig.Processors.InsertClusterAttributes.Attributes[0].Value)
		require.Equal(t, "insert", collectorConfig.Processors.InsertClusterAttributes.Attributes[1].Action)
		require.Equal(t, "cloud.provider", collectorConfig.Processors.InsertClusterAttributes.Attributes[1].Key)
		require.Equal(t, "test-cloud-provider", collectorConfig.Processors.InsertClusterAttributes.Attributes[1].Value)
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
}
