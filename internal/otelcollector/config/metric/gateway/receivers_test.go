package gateway

import (
	"context"
	"testing"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
				testutils.NewMetricPipelineBuilder().WithName("test").WithAnnotations(map[string]string{"experimental-kyma-input": "true"}).Build(),
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
		require.Len(t, kymaStatsReceiver.Modules, 1)
		require.Equal(t, "operator.kyma-project.io", kymaStatsReceiver.Modules[0].Group)
		require.Equal(t, "v1alpha1", kymaStatsReceiver.Modules[0].Version)
		require.Equal(t, "telemetries", kymaStatsReceiver.Modules[0].Resource)
	})
}
