package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestReceivers(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("OTLP receiver", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		otlpReceiver := collectorConfig.Receivers.OTLP
		require.NotNil(t, otlpReceiver)
		require.Equal(t, "${MY_POD_IP}:4318", otlpReceiver.Protocols.HTTP.Endpoint)
		require.Equal(t, "${MY_POD_IP}:4317", otlpReceiver.Protocols.GRPC.Endpoint)
	})

	t.Run("singleton kyma stats receiver creator", func(t *testing.T) {
		gatewayNamespace := "test-namespace"

		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithAnnotations(map[string]string{KymaInputAnnotation: "true"}).Build(),
			},
			BuildOptions{
				GatewayNamespace: gatewayNamespace,
				KymaInputAllowed: true,
			},
		)
		require.NoError(t, err)

		singletonKymaStatsReceiverCreator := collectorConfig.Receivers.SingletonKymaStatsReceiverCreator
		require.NotNil(t, singletonKymaStatsReceiverCreator)
		require.Equal(t, "serviceAccount", singletonKymaStatsReceiverCreator.AuthType)
		require.Equal(t, "telemetry-metric-gateway-kymastats", singletonKymaStatsReceiverCreator.LeaderElection.LeaseName)
		require.Equal(t, gatewayNamespace, singletonKymaStatsReceiverCreator.LeaderElection.LeaseNamespace)

		kymaStatsReceiver := singletonKymaStatsReceiverCreator.SingletonKymaStatsReceiver.KymaStatsReceiver
		require.Equal(t, "serviceAccount", kymaStatsReceiver.AuthType)
		require.Equal(t, "30s", kymaStatsReceiver.CollectionInterval)
		require.Len(t, kymaStatsReceiver.Resources, 1)
		require.Equal(t, "operator.kyma-project.io", kymaStatsReceiver.Resources[0].Group)
		require.Equal(t, "v1alpha1", kymaStatsReceiver.Resources[0].Version)
		require.Equal(t, "telemetries", kymaStatsReceiver.Resources[0].Resource)
	})

	t.Run("singleton k8s cluster receiver creator", func(t *testing.T) {
		gatewayNamespace := "test-namespace"
		expectedMetricsToDrop := K8sClusterMetricsConfig{
			K8sContainerStorageRequest:          MetricConfig{false},
			K8sContainerStorageLimit:            MetricConfig{false},
			K8sContainerEphemeralStorageRequest: MetricConfig{false},
			K8sContainerEphemeralStorageLimit:   MetricConfig{false},
			K8sContainerRestarts:                MetricConfig{false},
			K8sContainerReady:                   MetricConfig{false},
			K8sNamespacePhase:                   MetricConfig{false},
			K8sReplicationControllerAvailable:   MetricConfig{false},
			K8sReplicationControllerDesired:     MetricConfig{false},
		}
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").WithRuntimeInput(true).Build(),
			},
			BuildOptions{
				GatewayNamespace: gatewayNamespace,
			},
		)
		require.NoError(t, err)

		singletonK8sClusterReceiverCreator := collectorConfig.Receivers.SingletonK8sClusterReceiverCreator
		require.NotNil(t, singletonK8sClusterReceiverCreator)
		require.Equal(t, "serviceAccount", singletonK8sClusterReceiverCreator.AuthType)
		require.Equal(t, "telemetry-metric-gateway-k8scluster", singletonK8sClusterReceiverCreator.LeaderElection.LeaseName)
		require.Equal(t, gatewayNamespace, singletonK8sClusterReceiverCreator.LeaderElection.LeaseNamespace)

		k8sClusterReceiver := singletonK8sClusterReceiverCreator.SingletonK8sClusterReceiver.K8sClusterReceiver
		require.Equal(t, "serviceAccount", k8sClusterReceiver.AuthType)
		require.Equal(t, "30s", k8sClusterReceiver.CollectionInterval)
		require.Len(t, k8sClusterReceiver.NodeConditionsToReport, 1)
		require.Equal(t, []string{""}, k8sClusterReceiver.NodeConditionsToReport)
		require.Len(t, k8sClusterReceiver.AllocatableTypesToReport, 2)
		require.Equal(t, []string{"cpu", "memory"}, k8sClusterReceiver.AllocatableTypesToReport)
		require.Equal(t, expectedMetricsToDrop, k8sClusterReceiver.Metrics)
	})
}
