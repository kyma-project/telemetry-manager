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
		name              string
		goldenFileName    string
		tracePipelines    []telemetryv1beta1.TracePipeline
		logPipelines      []telemetryv1beta1.LogPipeline
		serviceEnrichment string
		moduleVersion     string
	}{
		{
			name:           "single trace pipeline",
			goldenFileName: "trace-single-pipeline.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test").Build(),
			},
		},
		{
			name:           "multiple trace pipelines",
			goldenFileName: "trace-multiple-pipelines.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace-1").Build(),
				testutils.NewTracePipelineBuilder().WithName("test-trace-2").WithOTLPOutput().Build(),
			},
		},
		{
			name:              "single trace pipeline with otel service enrichment",
			goldenFileName:    "trace-service-enrichment-otel.yaml",
			serviceEnrichment: "otel",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput().
					Build(),
			},
		},
		{
			name:           "trace pipeline with http protocol and custom path",
			goldenFileName: "trace-http-protocol-with-custom-path.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/traces"),
					).Build(),
			},
		},
		{
			name:           "trace pipeline with http protocol without custom path",
			goldenFileName: "trace-http-protocol-without-custom-path.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name:           "multiple trace pipelines with transforms",
			goldenFileName: "trace-user-defined-transforms.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
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
			name:           "multiple trace pipelines with filters",
			goldenFileName: "trace-user-defined-filters.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
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
			name:           "trace pipeline with transform and filter",
			goldenFileName: "trace-user-defined-transform-filter.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
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
			name:           "trace pipeline with OAuth2",
			goldenFileName: "trace-oauth2-authentication.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
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
		{
			name:           "single log pipeline",
			goldenFileName: "log-single-pipeline.yaml",
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test-log").WithOTLPOutput().Build(),
			},
		},
		{
			name:           "trace and log pipelines",
			goldenFileName: "trace-and-log-pipelines.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace").Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test-log").WithOTLPOutput().Build(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildOptions := BuildOptions{
				TracePipelines: tt.tracePipelines,
				LogPipelines:   tt.logPipelines,
				Cluster: common.ClusterOptions{
					ClusterName:   "${KUBERNETES_SERVICE_HOST}",
					CloudProvider: "test-cloud-provider",
				},
				ServiceEnrichment: tt.serviceEnrichment,
				ModuleVersion:     tt.moduleVersion,
			}

			config, _, err := sut.Build(context.Background(), buildOptions)
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
