package metricagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	ctx := context.Background()
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name                string
			goldenFileName      string
			pipeline            telemetryv1alpha1.MetricPipeline
			istioEnabled        bool
			overwriteGoldenFile bool
		}{
			{
				name:           "istio input only",
				goldenFileName: "istio-only.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(true).
					Build(),
			},
			{
				name:           "prometheus input only",
				goldenFileName: "prometheus-only.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(false).
					WithPrometheusInput(true).
					WithIstioInput(false).
					Build(),
			},
			{
				name:           "runtime input only",
				goldenFileName: "runtime-only.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
			},
			{
				name:           "istio module is not installed",
				goldenFileName: "istio-ops-disabled.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithPrometheusInput(true).
					WithIstioInput(false).
					WithIstioInputEnvoyMetrics(false).
					Build(),
			},
			{
				name:           "istio module is installed",
				goldenFileName: "istio-ops-enabled.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithPrometheusInput(true).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					Build(),
				istioEnabled: true,
			},
			{
				name:           "istio envoy metrics enabled",
				goldenFileName: "istio-envoy.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					Build(),
			},
			{
				name:           "runtime all resource metrics enabled",
				goldenFileName: "runtime-resources-all-enabled.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(true).
					WithRuntimeInputVolumeMetrics(true).
					WithRuntimeInputStatefulSetMetrics(true).
					WithRuntimeInputDeploymentMetrics(true).
					WithRuntimeInputDaemonSetMetrics(true).
					WithRuntimeInputJobMetrics(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
			},
			{
				name:           "runtime some resource metrics disabled",
				goldenFileName: "runtime-resources-some-disabled.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(false).
					WithRuntimeInputVolumeMetrics(false).
					WithRuntimeInputStatefulSetMetrics(true).
					WithRuntimeInputDeploymentMetrics(true).
					WithRuntimeInputDaemonSetMetrics(false).
					WithRuntimeInputJobMetrics(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
			},
			{
				name:           "comprehensive setup with all features enabled",
				goldenFileName: "setup-comprehensive.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(true).
					WithRuntimeInputVolumeMetrics(true).
					WithRuntimeInputStatefulSetMetrics(true).
					WithRuntimeInputDeploymentMetrics(true).
					WithRuntimeInputDaemonSetMetrics(true).
					WithRuntimeInputJobMetrics(true).
					WithPrometheusInput(true).
					WithPrometheusInputDiagnosticMetrics(true).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					Build(),
			},
		}

		buildOptions := BuildOptions{
			IstioCertPath:               "/etc/istio-output-certs",
			InstrumentationScopeVersion: "main",
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{tt.pipeline}, buildOptions)
				require.NoError(t, err)

				configYAML, err := yaml.Marshal(config)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				if tt.overwriteGoldenFile {
					err = os.WriteFile(goldenFilePath, configYAML, 0600)
					require.NoError(t, err, "failed to overwrite golden file")

					t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", tt.goldenFileName)
				}

				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
