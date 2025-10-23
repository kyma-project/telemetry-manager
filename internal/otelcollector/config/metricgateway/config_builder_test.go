package metricgateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeConfig(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{Reader: fakeClient}

	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.MetricPipeline
		goldenFileName      string
		overwriteGoldenFile bool
	}{
		{
			name:           "simple single pipeline setup",
			goldenFileName: "setup-simple.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITH custom 'Path' field",
			goldenFileName: "http-protocol-with-custom-path.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/metrics"),
					).Build(),
			},
		},
		{
			name:           "pipeline using http protocol WITHOUT custom 'Path' field",
			goldenFileName: "http-protocol-without-custom-path.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name:           "two pipelines with comprehensive configuration",
			goldenFileName: "setup-comprehensive.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithOTLPInput(true, testutils.IncludeNamespaces("apps-cls")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"resource.attributes[\"k8s.namespace.name\"] == \"production\""},
						Statements: []string{"set(attributes[\"environment\"], \"prod\")"},
					}).WithFilter(telemetryv1alpha1.FilterSpec{
					Conditions: []string{"metric.type == METRIC_DATA_TYPE_GAUGE"},
				}).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("dynatrace").
					WithOTLPInput(true, testutils.IncludeNamespaces("apps-dynatrace")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"resource.attributes[\"k8s.namespace.name\"] == \"staging\""},
						Statements: []string{"set(attributes[\"environment\"], \"staging\")"},
					}).WithFilter(telemetryv1alpha1.FilterSpec{
					Conditions: []string{"metric.type == METRIC_DATA_TYPE_SUMMARY"},
				}).Build(),
			},
		},
		{
			name:           "pipeline with OTLP input disabled",
			goldenFileName: "otlp-disabled.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithOTLPInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},

		{
			name:           "pipeline with OTLP input and namespace filters",
			goldenFileName: "otlp-namespace-filters.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithOTLPInput(true,
						testutils.IncludeNamespaces("monitoring", "observability"),
						testutils.ExcludeNamespaces("kube-system", "istio-system"),
					).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline with all inputs disabled except OTLP",
			goldenFileName: "otlp-only.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(false).
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "two pipelines with user-defined transforms",
			goldenFileName: "user-defined-transforms.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"log.level\"], \"error\")",
							"set(body, \"transformed1\")",
						},
					}).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("dynatrace").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"log.level\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).Build(),
			},
		},
		{
			name:           "two pipelines with user-defined Filter",
			goldenFileName: "user-defined-filters.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithFilter(telemetryv1alpha1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_SUMMARY"},
					}).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("dynatrace").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithFilter(telemetryv1alpha1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_HISTOGRAM"},
					}).Build(),
			},
		},
		{
			name:           "pipeline with user-defined Transform Filter",
			goldenFileName: "user-defined-transform-filter.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("cls").
					WithOTLPInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"log.level\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).
					WithFilter(telemetryv1alpha1.FilterSpec{
						Conditions: []string{"metric.type == METRIC_DATA_TYPE_SUMMARY"},
					}).Build(),
			},
		},
	}

	buildOptions := BuildOptions{
		ClusterName:   "${KUBERNETES_SERVICE_HOST}",
		CloudProvider: "test-cloud-provider",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, _, err := sut.Build(t.Context(), tt.pipelines, buildOptions)
			require.NoError(t, err)
			configYAML, err := yaml.Marshal(config)
			require.NoError(t, err, "failed to marshal config")

			goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
			if tt.overwriteGoldenFile {
				err = os.WriteFile(goldenFilePath, configYAML, 0600)
				require.NoError(t, err, "failed to overwrite golden file")

				t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", tt.goldenFileName)
			}

			goldenFile, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err, "failed to load golden file")

			require.NoError(t, err)
			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}
