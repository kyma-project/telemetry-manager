package tracegateway

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
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	tests := []struct {
		name           string
		pipelines      []telemetryv1beta1.TracePipeline
		goldenFileName string
	}{
		{
			name: "single pipeline",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test").Build(),
			},
			goldenFileName: "single-pipeline.yaml",
		},
		{
			name:           "pipeline using http protocol WITH custom 'Path' field",
			goldenFileName: "http-protocol-with-custom-path.yaml",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/traces"),
					).Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITHOUT custom 'Path' field",
			goldenFileName: "http-protocol-without-custom-path.yaml",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name: "two pipelines with user-defined transforms",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test1").
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed1\")",
						},
					}).Build(),
				testutils.NewTracePipelineBuilder().
					WithName("test2").
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).Build(),
			},
			goldenFileName: "user-defined-transforms.yaml",
		},
		{
			name: "pipeline with user-defined filters",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test1").
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(span.name, \".*grpc.*\")"},
					}).Build(),
				testutils.NewTracePipelineBuilder().
					WithName("test2").
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(spanevent.attributes[\"foo\"], \".*bar.*\")"},
					}).Build(),
			},
			goldenFileName: "user-defined-filters.yaml",
		},
		{
			name: "pipeline with user-defined transform and filters",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test1").
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(span.name, \".*grpc.*\")"},
					}).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).Build(),
			},
			goldenFileName: "user-defined-transform-filter.yaml",
		},
		{
			name: "pipeline using OAuth2 authentication",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).
					WithOAuth2(
						testutils.OAuth2ClientID("client-id"),
						testutils.OAuth2ClientSecret("client-secret"),
						testutils.OAuth2TokenURL("https://auth.example.com/oauth2/token"),
						testutils.OAuth2Scopes([]string{"traces"}),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, _, err := sut.Build(t.Context(), tt.pipelines, buildOptions)
			require.NoError(t, err)
			configYAML, err := yaml.Marshal(config)
			require.NoError(t, err, "failed to marshal config")

			goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
			if testutils.ShouldUpdateGoldenFiles() {
				testutils.UpdateGoldenFileYAML(t, goldenFilePath, configYAML)
				return
			}

			goldenFile, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err, "failed to load golden file")

			require.NoError(t, err)
			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}
