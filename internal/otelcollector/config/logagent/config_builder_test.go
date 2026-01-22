package logagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	sut := Builder{}

	tests := []struct {
		name           string
		goldenFileName string
		pipelines      []telemetryv1beta1.LogPipeline
	}{
		{
			name:           "single pipeline",
			goldenFileName: "single-pipeline.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithKeepOriginalBody(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
					Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITH custom 'Path' field",
			goldenFileName: "http-protocol-with-custom-path.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/logs"),
					).Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITHOUT custom 'Path' field",
			goldenFileName: "http-protocol-without-custom-path.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name:           "single pipeline with namespace included",
			goldenFileName: "single-pipeline-namespace-included.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true, testutils.IncludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "single pipeline with namespace excluded",
			goldenFileName: "single-pipeline-namespace-excluded.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true, testutils.ExcludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "two pipelines with user-defined transforms",
			goldenFileName: "user-defined-transforms.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithRuntimeInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test2").
					WithRuntimeInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed2\")"},
					}).
					Build(),
			},
		},
		{
			name:           "two pipelines with user-defined filter",
			goldenFileName: "user-defined-filters.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithRuntimeInput(true).
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test2").
					WithRuntimeInput(true).
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.body, \".*error.*\")"},
					}).
					Build(),
			},
		},
		{
			name:           "pipeline with user-defined transform and filter",
			goldenFileName: "user-defined-transform-filter.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithRuntimeInput(true).
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
					}).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).
					Build(),
			},
		},
		{
			name:           "pipeline using OAuth2 authentication",
			goldenFileName: "oauth2-authentication.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).
					WithOAuth2(
						testutils.OAuth2ClientID("client-id"),
						testutils.OAuth2ClientSecret("client-secret"),
						testutils.OAuth2TokenURL("https://auth.example.com/oauth2/token"),
						testutils.OAuth2Scopes([]string{"logs"}),
					).Build(),
			},
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
				testutils.UpdateGoldenFileYAML(t, goldenFilePath, configYAML)
				return
			}

			goldenFile, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err, "failed to load golden file")

			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}

// TestBuildConfigWithOtelServiceEnrichment verifies that the Log Agent config is built correctly
// when pipelines use OTel service enrichment strategy
// (temporary - will be removed after deprecation of legacy enrichment strategy since this will become default).
func TestBuildConfigWithOtelServiceEnrichment(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	goldenFileName := "service-enrichment-otel.yaml"

	pipelines := []telemetryv1beta1.LogPipeline{
		testutils.NewLogPipelineBuilder().
			WithName("test").
			WithRuntimeInput(true).
			WithKeepOriginalBody(true).
			WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).
			Build(),
	}

	buildOptions := BuildOptions{
		Cluster: common.ClusterOptions{
			ClusterName:   "test-cluster",
			CloudProvider: "azure",
		},
		InstrumentationScopeVersion: "main",
		AgentNamespace:              "kyma-system",
		ServiceEnrichment:           commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
	}

	config, _, err := sut.Build(t.Context(), pipelines, buildOptions)
	require.NoError(t, err)
	configYAML, err := yaml.Marshal(config)
	require.NoError(t, err, "failed to marshal config")

	goldenFilePath := filepath.Join("testdata", goldenFileName)
	if testutils.ShouldUpdateGoldenFiles() {
		testutils.UpdateGoldenFileYAML(t, goldenFilePath, configYAML)
		return
	}

	goldenFile, err := os.ReadFile(goldenFilePath)
	require.NoError(t, err, "failed to load golden file")

	require.NoError(t, err)
	require.Equal(t, string(goldenFile), string(configYAML))
}
