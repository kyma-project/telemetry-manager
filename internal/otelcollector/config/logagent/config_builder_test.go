package logagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	sut := Builder{}

	tests := []struct {
		name           string
		pipelines      []telemetryv1alpha1.LogPipeline
		goldenFileName string
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
			name:           "pipeline using http protocol WITH custom 'Path' field",
			goldenFileName: "http-protocol-with-custom-path.yaml",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithApplicationInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/logs"),
					).Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITHOUT custom 'Path' field",
			goldenFileName: "http-protocol-without-custom-path.yaml",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithApplicationInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
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
			goldenFileName: "user-defined-transforms.yaml",
		},
		{
			name: "two pipelines with user-defined filter",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithApplicationInput(true).
					WithOTLPOutput().
					WithFilter(telemetryv1alpha1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test2").
					WithApplicationInput(true).
					WithOTLPOutput().
					WithFilter(telemetryv1alpha1.FilterSpec{
						Conditions: []string{"IsMatch(log.body, \".*error.*\")"},
					}).
					Build(),
			},
			goldenFileName: "user-defined-filters.yaml",
		},
		{
			name: "pipeline with user-defined transform and filter",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithApplicationInput(true).
					WithOTLPOutput().
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
					}).
					WithFilter(telemetryv1alpha1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).
					Build(),
			},
			goldenFileName: "user-defined-transform-filter.yaml",
		},
	}

	buildOptions := BuildOptions{
		Cluster: common.ClusterOptions{
			ClusterName:   "test-cluster",
			CloudProvider: "azure",
		},
		InstrumentationScopeVersion: "main",
		AgentNamespace:              "kyma-system",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectorConfig, _, err := sut.Build(t.Context(), tt.pipelines, buildOptions)
			require.NoError(t, err)
			configYAML, err := yaml.Marshal(collectorConfig)
			require.NoError(t, err, "failed to marshal config")

			goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
			if testutils.ShouldUpdateGoldenFiles() {
				testutils.UpdateGoldenFile(t, goldenFilePath, configYAML)
				return
			}

			goldenFile, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err, "failed to load golden file")

			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}
