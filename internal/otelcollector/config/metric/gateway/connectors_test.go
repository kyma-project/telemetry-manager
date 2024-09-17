package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestConnectors(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("forward connector", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Connectors, "forward/test")
		require.Equal(t, struct{}{}, collectorConfig.Connectors["forward/test"])
	})

	t.Run("routing connector", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(
			ctx,
			[]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").Build(),
			},
			BuildOptions{},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Connectors, "routing/test")
		expectedRoutingConnector := RoutingConnector{
			DefaultPipelines: []string{"metrics/test-attributes-enrichment"},
			ErrorMode:        "ignore",
			Table: []RoutingConnectorTableEntry{
				{
					Statement: "route() where attributes[\"io.kyma-project.telemetry.skip_enrichment\"] == \"true\"",
					Pipelines: []string{"metrics/test-output"},
				},
			},
		}
		require.Equal(t, expectedRoutingConnector, collectorConfig.Connectors["routing/test"])
	})
}
