package logagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	sut := Builder{}

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
					WithApplicationInput(true).
					WithKeepOriginalBody(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
					Build(),
			},
			goldenFileName: "single-pipeline.yaml",
		},
		{
			name: "single pipeline with namespace included",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithApplicationInput(true, testutils.ExtIncludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			goldenFileName: "single-pipeline-namespace-included.yaml",
		},
		{
			name: "single pipeline with namespace excluded",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithApplicationInput(true, testutils.ExtExcludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			goldenFileName: "single-pipeline-namespace-excluded.yaml",
		},
		{
			name: "two pipelines with user-defined transforms",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithApplicationInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test2").
					WithApplicationInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
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
		InstrumentationScopeVersion: "main",
		AgentNamespace:              "kyma-system",
		CloudProvider:               "azure",
		ClusterName:                 "test-cluster",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectorConfig, _, err := sut.Build(t.Context(), tt.pipelines, buildOptions)
			require.NoError(t, err)
			configYAML, err := yaml.Marshal(collectorConfig)
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
}
