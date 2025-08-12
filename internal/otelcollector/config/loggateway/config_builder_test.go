package loggateway

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(ctx, []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
		}, BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		})
		require.NoError(t, err)

		const endpointEnvVar = "OTLP_ENDPOINT_TEST"

		expectedEndpoint := fmt.Sprintf("${%s}", endpointEnvVar)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")
		otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, otlpExporterConfig.OTLP.Endpoint)

		require.Contains(t, envVars, endpointEnvVar)
		require.Equal(t, "http://localhost", string(envVars[endpointEnvVar]))
	})

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name                string
			pipelines           []telemetryv1alpha1.LogPipeline
			goldenFileName      string
			overwriteGoldenFile bool
		}{
			{
				name: "single pipeline",
				pipelines: []telemetryv1alpha1.LogPipeline{
					testutils.NewLogPipelineBuilder().
						WithName("test").
						WithOTLPOutput().
						Build(),
				},
				goldenFileName: "single-pipeline.yaml",
			},
			{
				name: "two pipelines with user-defined transforms",
				pipelines: []telemetryv1alpha1.LogPipeline{
					testutils.NewLogPipelineBuilder().
						WithName("test1").
						WithOTLPOutput().
						WithTransform(telemetryv1alpha1.TransformSpec{
							Conditions: []string{"IsMatch(body, \".*error.*\")"},
							Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
						}).
						Build(),
					testutils.NewLogPipelineBuilder().
						WithName("test2").
						WithOTLPOutput().
						WithTransform(telemetryv1alpha1.TransformSpec{
							Conditions: []string{"IsMatch(body, \".*error.*\")"},
							Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed2\")"},
						}).
						Build(),
				},
				goldenFileName: "two-pipelines-with-transforms.yaml",
			},
		}

		buildOptions := BuildOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
			ModuleVersion: "1.0.0",
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				collectorConfig, _, err := sut.Build(ctx, tt.pipelines, buildOptions)
				require.NoError(t, err)
				configYAML, err := yaml.Marshal(collectorConfig)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				if tt.overwriteGoldenFile {
					err = os.WriteFile(goldenFilePath, configYAML, 0600)
					require.NoError(t, err, "failed to overwrite golden file")

					t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", goldenFilePath)

					return
				}

				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
