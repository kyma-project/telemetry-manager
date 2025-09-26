package metricagent

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

func TestBuildConfig(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{
		Reader: fakeClient,
	}

	tests := []struct {
		name                string
		goldenFileName      string
		pipelines           []telemetryv1alpha1.MetricPipeline
		istioEnabled        bool
		overwriteGoldenFile bool
	}{
		{
			name:           "pipeline with istio input only",
			goldenFileName: "istio-only.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(true).
					Build(),
			},
		},
		{
			name:           "pipeline with prometheus input only",
			goldenFileName: "prometheus-only.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(false).
					WithPrometheusInput(true).
					WithIstioInput(false).
					Build(),
			},
		},
		{
			name:           "pipeline with runtime input only",
			goldenFileName: "runtime-only.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
			},
		},
		{
			name:           "istio module is not installed",
			goldenFileName: "istio-ops-disabled.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithPrometheusInput(true).
					WithIstioInput(false).
					WithIstioInputEnvoyMetrics(false).
					Build(),
			},
		},
		{
			name:           "istio module is installed",
			goldenFileName: "istio-ops-enabled.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithPrometheusInput(true).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					Build(),
			},
			istioEnabled: true,
		},
		{
			name:           "pipeline with istio envoy metrics enabled",
			goldenFileName: "istio-envoy.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					Build(),
			},
		},
		{
			name:           "pipeline with istio diagnostic metrics",
			goldenFileName: "istio-diagnostic.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithIstioInput(true).
					WithIstioInputDiagnosticMetrics(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline with prometheus diagnostic metrics",
			goldenFileName: "prometheus-diagnostic.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithPrometheusInput(true).
					WithPrometheusInputDiagnosticMetrics(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline with all runtime input resources enabled",
			goldenFileName: "runtime-resources-all-enabled.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(true).
					WithRuntimeInputVolumeMetrics(true).
					WithRuntimeInputStatefulSetMetrics(true).
					WithRuntimeInputDeploymentMetrics(true).
					WithRuntimeInputDaemonSetMetrics(true).
					WithRuntimeInputJobMetrics(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
			},
		},
		{
			name:           "pipeline with all runtime input resources disabled",
			goldenFileName: "runtime-resources-all-disabled.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputNodeMetrics(false).
					WithRuntimeInputVolumeMetrics(false).
					WithRuntimeInputDeploymentMetrics(false).
					WithRuntimeInputDaemonSetMetrics(false).
					WithRuntimeInputStatefulSetMetrics(false).
					WithRuntimeInputJobMetrics(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline with some runtime input resources disabled",
			goldenFileName: "runtime-resources-some-disabled.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(false).
					WithRuntimeInputVolumeMetrics(false).
					WithRuntimeInputStatefulSetMetrics(true).
					WithRuntimeInputDeploymentMetrics(true).
					WithRuntimeInputDaemonSetMetrics(false).
					WithRuntimeInputJobMetrics(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
			},
		},
		{
			name:           "pipeline using HTTP WITH custom 'Path' field",
			goldenFileName: "http-with-custom-path.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
						testutils.OTLPEndpointPath("v2/otlp/v1/metrics"),
					).Build(),
			},
		},
		{
			name:           "pipeline using HTTP WITHOUT custom 'Path' field",
			goldenFileName: "http-without-custom-path.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true).
					WithOTLPOutput(
						testutils.OTLPProtocol("http"),
					).Build(),
			},
		},
		{
			name:           "complex pipeline with comprehensive configuration",
			goldenFileName: "setup-comprehensive.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true, testutils.IncludeNamespaces("production", "staging")).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(true).
					WithPrometheusInput(true, testutils.ExcludeNamespaces("kube-system")).
					WithPrometheusInputDiagnosticMetrics(true).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					WithOTLPInput(true, testutils.IncludeNamespaces("apps")).
					WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"resource.attributes[\"k8s.namespace.name\"] == \"production\""},
						Statements: []string{"set(attributes[\"environment\"], \"prod\")"},
					}).Build(),
			},
		},
		{
			name:           "pipeline with runtime input and namespace filters",
			goldenFileName: "runtime-namespace-filters.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithRuntimeInput(true,
						testutils.IncludeNamespaces("monitoring", "observability"),
						testutils.ExcludeNamespaces("kube-system", "istio-system"),
					).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline with prometheus input and namespace filters",
			goldenFileName: "prometheus-namespace-filters.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithPrometheusInput(true,
						testutils.IncludeNamespaces("monitoring", "observability"),
						testutils.ExcludeNamespaces("kube-system", "istio-system"),
					).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		},
		{
			name:           "pipeline with istio input and namespace filters",
			goldenFileName: "istio-namespace-filters.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test").
					WithIstioInput(true,
						testutils.IncludeNamespaces("monitoring", "observability"),
						testutils.ExcludeNamespaces("kube-system", "istio-system"),
					).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).Build(),
			},
		}, {
			name:           "three pipelines with multiple input types and mixed configurations",
			goldenFileName: "multiple-inputs-mixed.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test1").
					WithRuntimeInput(true, testutils.IncludeNamespaces("default")).
					WithPrometheusInput(true, testutils.ExcludeNamespaces("kube-system")).
					WithIstioInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://foo")).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test2").
					WithRuntimeInput(false).
					WithPrometheusInput(false).
					WithIstioInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://foo")).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test3").
					WithRuntimeInput(true).
					WithPrometheusInput(false).
					WithIstioInput(false).
					WithOTLPOutput(testutils.OTLPEndpoint("https://bar")).Build(),
			},
		},
		{
			name:           "two pipelines with user-defined transforms",
			goldenFileName: "user-defined-transforms.yaml",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithName("test1").
					WithRuntimeInput(true).
					WithOTLPOutput(testutils.OTLPEndpoint("https://localhost")).
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"log.level\"], \"error\")",
							"set(body, \"transformed1\")",
						},
					}).Build(),
				testutils.NewMetricPipelineBuilder().
					WithName("test2").
					WithRuntimeInput(true).
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
	}

	buildOptions := BuildOptions{
		IstioCertPath:               "/etc/istio-output-certs",
		InstrumentationScopeVersion: "main",
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

			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}

func TestBuildConfigShuffled(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()

	sut := Builder{
		Reader: fakeClient,
	}

	buildOptions := BuildOptions{
		IstioCertPath:               "/etc/istio-output-certs",
		InstrumentationScopeVersion: "main",
	}

	pipelines := []telemetryv1alpha1.MetricPipeline{
		testutils.NewMetricPipelineBuilder().
			WithName("test1").
			WithRuntimeInput(true, testutils.IncludeNamespaces("default")).
			WithPrometheusInput(true, testutils.ExcludeNamespaces("kube-system")).
			WithIstioInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint("https://foo")).Build(),
		testutils.NewMetricPipelineBuilder().
			WithName("test2").
			WithRuntimeInput(false).
			WithPrometheusInput(false).
			WithIstioInput(true).
			WithOTLPOutput(testutils.OTLPEndpoint("https://foo")).Build(),
		testutils.NewMetricPipelineBuilder().
			WithName("test3").
			WithRuntimeInput(true).
			WithPrometheusInput(false).
			WithIstioInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint("https://bar")).Build(),
	}

	config1, _, err := sut.Build(t.Context(), []telemetryv1alpha1.MetricPipeline{pipelines[0], pipelines[1], pipelines[2]}, buildOptions)
	require.NoError(t, err)

	config2, _, err := sut.Build(t.Context(), []telemetryv1alpha1.MetricPipeline{pipelines[1], pipelines[0], pipelines[2]}, buildOptions)
	require.NoError(t, err)

	config3, _, err := sut.Build(t.Context(), []telemetryv1alpha1.MetricPipeline{pipelines[2], pipelines[1], pipelines[0]}, buildOptions)
	require.NoError(t, err)

	config1YAML, err := yaml.Marshal(config1)
	require.NoError(t, err, "failed to marshal config1")

	config2YAML, err := yaml.Marshal(config2)
	require.NoError(t, err, "failed to marshal config2")

	config3YAML, err := yaml.Marshal(config3)
	require.NoError(t, err, "failed to marshal config3")

	require.Equal(t, string(config1YAML), string(config2YAML), "config1 and config2 should be equal regardless of pipeline order")
	require.Equal(t, string(config2YAML), string(config3YAML), "config2 and config3 should be equal regardless of pipeline order")
	require.Equal(t, string(config1YAML), string(config3YAML), "config1 and config3 should be equal regardless of pipeline order")
}
