package otlpgateway

import (
	"context"
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

func TestBuild(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	tests := []struct {
		name           string
		goldenFileName string
		tracePipelines []telemetryv1beta1.TracePipeline
	}{
		{
			name:           "empty pipelines",
			goldenFileName: "empty-pipelines.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{},
		},
		{
			name:           "single trace pipeline",
			goldenFileName: "single-trace-pipeline.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace").Build(),
			},
		},
		{
			name:           "multiple trace pipelines",
			goldenFileName: "multiple-trace-pipelines.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace-1").Build(),
				testutils.NewTracePipelineBuilder().WithName("test-trace-2").WithOTLPOutput().Build(),
			},
		},
		{
			name:           "trace pipeline with http protocol",
			goldenFileName: "trace-http-protocol.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v1/traces"),
					).Build(),
			},
		},
		{
			name:           "trace pipeline with transform",
			goldenFileName: "trace-transform.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
						},
					}).Build(),
			},
		},
		{
			name:           "trace pipeline with filter",
			goldenFileName: "trace-filter.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(span.name, \".*grpc.*\")"},
					}).Build(),
			},
		},
		{
			name:           "trace pipeline with OAuth2",
			goldenFileName: "trace-oauth2.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithOTLPOutput(testutils.OTLPProtocol("http")).
					WithOAuth2(
						testutils.OAuth2ClientID("client-id"),
						testutils.OAuth2ClientSecret("client-secret"),
						testutils.OAuth2TokenURL("https://auth.example.com/oauth2/token"),
						testutils.OAuth2Scopes([]string{"traces"}),
					).Build(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildOptions := BuildOptions{
				Cluster: common.ClusterOptions{
					ClusterName:   "${KUBERNETES_SERVICE_HOST}",
					CloudProvider: "test-cloud-provider",
				},
			}

			config, _, err := sut.Build(context.Background(), tt.tracePipelines, buildOptions)
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
			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}
