package loggateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	tests := []struct {
		name           string
		pipelines      []telemetryv1beta1.LogPipeline
		goldenFileName string
	}{
		{
			name: "single pipeline",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithOTLPOutput().
					Build(),
			},
			goldenFileName: "single-pipeline.yaml",
		},
		{
			name:           "pipeline using http protocol WITH custom 'Path' field",
			goldenFileName: "http-protocol-with-custom-path.yaml",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
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
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name: "single pipeline with OTLP disabled",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithOTLPInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			goldenFileName: "single-pipeline-otlp-disabled.yaml",
		},
		{
			name: "single pipeline with namespace included",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithOTLPInput(true, testutils.IncludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			goldenFileName: "single-pipeline-namespace-included.yaml",
		},
		{
			name: "single pipeline with namespace excluded",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test").
					WithOTLPInput(true, testutils.ExcludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
			goldenFileName: "single-pipeline-namespace-excluded.yaml",
		},
		{
			name: "two pipelines with user-defined transforms",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test2").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed2\")"},
					}).
					Build(),
			},
			goldenFileName: "user-defined-transforms.yaml",
		},
		{
			name: "two pipelines with user-defined filter",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test2").
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.body, \".*error.*\")"},
					}).
					Build(),
			},
			goldenFileName: "user-defined-filters.yaml",
		},
		{
			name: "pipeline with user-defined transform and filter",
			pipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test1").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed2\")"},
					}).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).Build(),
			},
			goldenFileName: "user-defined-transform-filter.yaml",
		},
		{
			name: "pipeline using OAuth2 authentication",
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
			goldenFileName: "oauth2-authentication.yaml",
		},
	}

	buildOptions := BuildOptions{
		Cluster: common.ClusterOptions{
			ClusterName:   "${KUBERNETES_SERVICE_HOST}",
			CloudProvider: "test-cloud-provider",
		},
		ModuleVersion: "1.0.0",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, tt.pipelines, buildOptions)
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
