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
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	tests := []struct {
		name              string
		goldenFileName    string
		pipelines         []telemetryv1beta1.TracePipeline
		serviceEnrichment string
	}{
		{
			name:           "single pipeline",
			goldenFileName: "single-pipeline.yaml",
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test").Build(),
			},
		},
		{
			name:              "single pipeline with otel service enrichment",
			goldenFileName:    "service-enrichment-otel.yaml",
			serviceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
			pipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput().
					Build(),
			},
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
			name:           "two pipelines with user-defined transforms",
			goldenFileName: "user-defined-transforms.yaml",
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
		},
		{
			name:           "pipeline with user-defined filters",
			goldenFileName: "user-defined-filters.yaml",
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
		},
		{
			name:           "pipeline with user-defined transform and filters",
			goldenFileName: "user-defined-transform-filter.yaml",
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
		},
		{
			name:           "pipeline using OAuth2 authentication",
			goldenFileName: "oauth2-authentication.yaml",
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildOptions := BuildOptions{
				Cluster: common.ClusterOptions{
					ClusterName:   "${KUBERNETES_SERVICE_HOST}",
					CloudProvider: "test-cloud-provider",
				},
				ServiceEnrichment: tt.serviceEnrichment,
			}

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
			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}
