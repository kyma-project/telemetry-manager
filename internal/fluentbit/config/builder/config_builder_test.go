package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeConfig(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-host-secret",
				Namespace: "test-host-secret-namespace",
			},
			Data: map[string][]byte{
				"test-host-secret-key": []byte("test-host-secret-value"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-var-secret",
				Namespace: "test-var-secret-namespace",
			},
			Data: map[string][]byte{
				"test-var-secret-key": []byte("test-var-secret-value"),
			},
		}).Build()
	sut := NewFluentBitConfigBuilder(fakeClient)

	t.Run("should return the correct fluent-bit configuration", func(t *testing.T) {
		pipelines := []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPHostFromSecret("test-host-secret", "test-host-secret-namespace", "test-host-secret-key"),
					testutils.HTTPClientTLSFromString("test-ca", "test-cert", "test-key"),
				).
				WithFile("test-file", "test-filecontent").
				WithVariable("test-var", "test-var-secret", "test-var-secret-namespace", "test-var-secret-key").
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines)
		require.NoError(t, err)

		sectionsConfig, err := BuildFluentBitSectionsConfig(&pipelines[0], sut.BuilderConfig)
		require.NoError(t, err)

		envConfigSecretKey := FormatEnvVarName("test-pipeline", "test-host-secret-namespace", "test-host-secret", "test-host-secret-key")

		expectedConfig := &FluentBitConfig{
			SectionsConfig: map[string]string{"test-pipeline.conf": sectionsConfig},
			FilesConfig:    map[string]string{"test-file": "test-filecontent"},
			EnvConfigSecret: map[string][]byte{
				envConfigSecretKey: []byte("test-host-secret-value"),
				"test-var":         []byte("test-var-secret-value"),
			},
			TLSConfigSecret: map[string][]byte{
				"test-pipeline-ca.crt":   []byte("test-ca"),
				"test-pipeline-cert.crt": []byte("test-cert"),
				"test-pipeline-key.key":  []byte("test-key"),
			},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})
}
