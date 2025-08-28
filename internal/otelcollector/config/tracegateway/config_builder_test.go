package tracegateway

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
	sut := Builder{Reader: fakeClient}

	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.TracePipeline
		goldenFileName      string
		overwriteGoldenFile bool
	}{
		{
			name: "single pipeline",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("test").Build(),
			},
			goldenFileName: "single-pipeline.yaml",
		},
		{
			name: "two pipelines with user-defined transforms",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithName("test1").
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed1\")",
						},
					}).Build(),
				testutils.NewTracePipelineBuilder().
					WithName("test2").
					WithTransform(telemetryv1alpha1.TransformSpec{
						Conditions: []string{"IsMatch(body, \".*error.*\")"},
						Statements: []string{
							"set(attributes[\"trace.status_code\"], \"error\")",
							"set(body, \"transformed2\")",
						},
					}).Build(),
			},
			goldenFileName: "two-pipelines-with-transforms.yaml",
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

				t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", goldenFilePath)

				return
			}

			goldenFile, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err, "failed to load golden file")

			require.NoError(t, err)
			require.Equal(t, string(goldenFile), string(configYAML))
		})
	}
}
