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
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
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
		metricPipelines   []telemetryv1beta1.MetricPipeline
		serviceEnrichment string
		moduleVersion     string
	}{
		{
			name:           "single pipeline",
			goldenFileName: "single-pipeline.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace").Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test-log").WithOTLPOutput().Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-metric").WithOTLPInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "trace-single pipeline only",
			goldenFileName: "trace-single-pipeline.yaml",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace").Build(),
			},
		},
		{
			name:           "log-single pipeline only",
			goldenFileName: "log-single-pipeline.yaml",
			moduleVersion:  "1.0.0",
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test-log").WithOTLPOutput().Build(),
			},
		},
		{
			name:           "metric-single pipeline only",
			goldenFileName: "metric-single-pipeline.yaml",
			moduleVersion:  "1.0.0",
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test-metric").WithOTLPInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:              "single pipeline with otel service enrichment",
			goldenFileName:    "service-enrichment-otel.yaml",
			serviceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
			moduleVersion:     "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithOTLPOutput().
					Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPOutput().
					Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITH custom 'Path' field",
			goldenFileName: "http-protocol-with-custom-path.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/traces"),
					).Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/logs"),
					).Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/metrics"),
					).Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITHOUT custom 'Path' field",
			goldenFileName: "http-protocol-without-custom-path.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name:           "two pipelines with user-defined transforms",
			goldenFileName: "user-defined-transforms.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace-1").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed1\")",
						},
					}).Build(),
				testutils.NewTracePipelineBuilder().
					WithName("test-trace-2").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log-1").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test-log-2").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed2\")"},
					}).
					Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric-1").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"metric.status\"], \"error\")",
							"set(body, \"transformed1\")",
						},
					}).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric-2").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"metric.status\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).Build(),
			},
		},
		{
			name:           "two pipelines with user-defined filter",
			goldenFileName: "user-defined-filters.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace-1").
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(span.name, \".*grpc.*\")"},
					}).Build(),
				testutils.NewTracePipelineBuilder().
					WithName("test-trace-2").
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(spanevent.attributes[\"foo\"], \".*bar.*\")"},
					}).Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log-1").
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithName("test-log-2").
					WithOTLPOutput().
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.body, \".*error.*\")"},
					}).
					Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric-1").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_SUMMARY"},
					}).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric-2").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_HISTOGRAM"},
					}).Build(),
			},
		},
		{
			name:           "pipeline with user-defined transform and filter",
			goldenFileName: "user-defined-transform-filter.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
					WithOTLPOutput().
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
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPOutput().
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed2\")"},
					}).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"IsMatch(log.attributes[\"foo\"], \".*bar.*\")"},
					}).Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"metric.status\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_SUMMARY"},
					}).Build(),
			},
		},
		{
			name:           "pipeline using OAuth2 authentication",
			goldenFileName: "oauth2-authentication.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test-trace").
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
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
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
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).
					WithOAuth2(
						testutils.OAuth2ClientID("client-id"),
						testutils.OAuth2ClientSecret("client-secret"),
						testutils.OAuth2TokenURL("https://auth.example.com/oauth2/token"),
						testutils.OAuth2Scopes([]string{"metrics"}),
					).Build(),
			},
		},
		{
			name:           "log-pipeline with OTLP input disabled",
			goldenFileName: "log-otlp-input-disabled.yaml",
			moduleVersion:  "1.0.0",
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "log-pipeline with namespace included",
			goldenFileName: "log-namespace-included.yaml",
			moduleVersion:  "1.0.0",
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPInput(true, testutils.IncludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "log-pipeline with namespace excluded",
			goldenFileName: "log-namespace-excluded.yaml",
			moduleVersion:  "1.0.0",
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithName("test-log").
					WithOTLPInput(true, testutils.ExcludeNamespaces("kyma-system", "default")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "mixed pipelines",
			goldenFileName: "mixed-pipelines.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test-trace").WithOTLPOutput().Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test-log-1").WithOTLPOutput().Build(),
				testutils.NewLogPipelineBuilder().WithName("test-log-2").WithOTLPOutput().Build(),
			},
		},
		// Metric-specific test cases
		{
			name:           "metric-pipeline with OTLP input disabled",
			goldenFileName: "metric-otlp-disabled.yaml",
			moduleVersion:  "1.0.0",
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					Build(),
			},
		},
		{
			name:           "metric-pipeline with namespace filters",
			goldenFileName: "metric-namespace-filters.yaml",
			moduleVersion:  "1.0.0",
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true, testutils.IncludeNamespaces("monitoring", "observability"), testutils.ExcludeNamespaces("kube-system", "istio-system")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					Build(),
			},
		},
		{
			name:           "metric-pipeline with OTLP only input",
			goldenFileName: "metric-otlp-only.yaml",
			moduleVersion:  "1.0.0",
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test-metric").
					WithOTLPInput(true).
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					Build(),
			},
		},
		{
			name:           "metric-comprehensive setup",
			goldenFileName: "metric-comprehensive.yaml",
			moduleVersion:  "1.0.0",
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithOTLPInput(true, testutils.IncludeNamespaces("apps-cls")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"metric.status\"], \"error\")",
							"set(body, \"transformed-cls\")",
						},
					}).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_SUMMARY"},
					}).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("dynatrace").
					WithOTLPInput(true, testutils.IncludeNamespaces("apps-dynatrace")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1beta1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"metric.status\"], \"error\")",
							"set(body, \"transformed-dynatrace\")",
						},
					}).
					WithFilter(telemetryv1beta1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_HISTOGRAM"},
					}).
					Build(),
			},
		},
		{
			name:           "all-signals-multi-backend",
			goldenFileName: "all-signals-multi-backend.yaml",
			moduleVersion:  "1.0.0",
			tracePipelines: []telemetryv1beta1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("trace-backend-1").WithOTLPOutput(testutils.OTLPEndpoint("https://backend-1.example.com")).Build(),
				testutils.NewTracePipelineBuilder().WithName("trace-backend-2").WithOTLPOutput(testutils.OTLPEndpoint("https://backend-2.example.com")).Build(),
			},
			logPipelines: []telemetryv1beta1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("log-backend-1").WithOTLPOutput(testutils.OTLPEndpoint("https://backend-1.example.com")).Build(),
				testutils.NewLogPipelineBuilder().WithName("log-backend-2").WithOTLPOutput(testutils.OTLPEndpoint("https://backend-2.example.com")).Build(),
			},
			metricPipelines: []telemetryv1beta1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("metric-backend-1").WithOTLPInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://backend-1.example.com")).Build(),
				testutils.NewMetricPipelineBuilder().WithName("metric-backend-2").WithOTLPInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://backend-2.example.com")).Build(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildOptions := BuildOptions{
				TracePipelines:  tt.tracePipelines,
				LogPipelines:    tt.logPipelines,
				MetricPipelines: tt.metricPipelines,
				Cluster: common.ClusterOptions{
					ClusterName:   "${KUBERNETES_SERVICE_HOST}",
					CloudProvider: "test-cloud-provider",
				},
				ServiceEnrichment: tt.serviceEnrichment,
				ModuleVersion:     tt.moduleVersion,
				GatewayNamespace:  "kyma-system",
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
