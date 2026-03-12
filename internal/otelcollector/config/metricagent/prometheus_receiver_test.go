package metricagent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestPrometheusReceiverConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{
		Reader: fakeClient,
	}

	t.Run("prometheus input enabled", func(t *testing.T) {
		tests := []struct {
			name                      string
			istioEnabled              bool
			expectedPodScrapeJobs     []string
			expectedServiceScrapeJobs []string
		}{
			{
				name: "istio not enabled",
				expectedPodScrapeJobs: []string{
					"app-pods",
				},
				expectedServiceScrapeJobs: []string{
					"app-services",
				},
			},
			{
				name:         "istio enabled",
				istioEnabled: true,
				expectedPodScrapeJobs: []string{
					"app-pods",
				},
				expectedServiceScrapeJobs: []string{
					"app-services",
					"app-services-secure",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				collectorConfig, _, err := sut.Build(ctx, []telemetryv1beta1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
				}, BuildOptions{
					IstioActive: tt.istioEnabled,
				})
				require.NoError(t, err)

				require.NotContains(t, collectorConfig.Receivers, "kubeletstats")
				require.NotContains(t, collectorConfig.Receivers, "prometheus/istio")

				require.Contains(t, collectorConfig.Receivers, "prometheus/app-pods")
				prometheusAppPods := collectorConfig.Receivers["prometheus/app-pods"].(*PrometheusReceiverConfig)
				require.Len(t, prometheusAppPods.Prometheus.ScrapeConfigs, len(tt.expectedPodScrapeJobs))

				for i := range prometheusAppPods.Prometheus.ScrapeConfigs {
					require.Equal(t, tt.expectedPodScrapeJobs[i], prometheusAppPods.Prometheus.ScrapeConfigs[i].JobName)
				}

				require.Contains(t, collectorConfig.Receivers, "prometheus/app-services")
				prometheusAppServices := collectorConfig.Receivers["prometheus/app-services"].(*PrometheusReceiverConfig)
				require.Len(t, prometheusAppServices.Prometheus.ScrapeConfigs, len(tt.expectedServiceScrapeJobs))

				for i := range prometheusAppServices.Prometheus.ScrapeConfigs {
					require.Equal(t, tt.expectedServiceScrapeJobs[i], prometheusAppServices.Prometheus.ScrapeConfigs[i].JobName)
				}
			})
		}
	})

	t.Run("istio input enabled", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithIstioInput(true).Build(),
		}, BuildOptions{
			IstioActive: true,
		})
		require.NoError(t, err)

		require.NotContains(t, collectorConfig.Receivers, "kubeletstats")
		require.NotContains(t, collectorConfig.Receivers, "prometheus/app-pods")
		require.Contains(t, collectorConfig.Receivers, "prometheus/istio")
		prometheusIstio := collectorConfig.Receivers["prometheus/istio"].(*PrometheusReceiverConfig)
		require.Len(t, prometheusIstio.Prometheus.ScrapeConfigs, 1)
		require.Len(t, prometheusIstio.Prometheus.ScrapeConfigs[0].KubernetesDiscoveryConfigs, 1)
	})

	t.Run("istio input envoy metrics enabled", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithIstioInput(true).WithIstioInputEnvoyMetrics(true).Build(),
		}, BuildOptions{
			IstioActive: true,
		})
		require.NoError(t, err)

		require.NotContains(t, collectorConfig.Receivers, "kubeletstats")
		require.NotContains(t, collectorConfig.Receivers, "prometheus/app-pods")
		require.Contains(t, collectorConfig.Receivers, "prometheus/istio")
		prometheusIstio := collectorConfig.Receivers["prometheus/istio"].(*PrometheusReceiverConfig)
		require.Len(t, prometheusIstio.Prometheus.ScrapeConfigs[0].MetricRelabelConfigs, 1)
		require.Equal(t, prometheusIstio.Prometheus.ScrapeConfigs[0].MetricRelabelConfigs[0].Regex, "envoy_.*|istio_.*")
	})
}
