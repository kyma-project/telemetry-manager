package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

const (
	hostSecretName  = "test-host-secret"
	hostSecretKey   = "test-host-secret-key"
	hostSecretValue = "test-host-secret-value"

	basicAuthSecretName    = "test-basic-auth-secret" // #nosec G101
	basicAuthUserKey       = "test-basic-auth-user-key"
	basicAuthUserValue     = "test-basic-auth-user-value"
	basicAuthPasswordKey   = "test-basic-auth-password-key"   // #nosec G101
	basicAuthPasswordValue = "test-basic-auth-password-value" // #nosec G101

	varSecretName  = "test-var-secret"
	varSecretKey   = "test-var-secret-key"
	varSecretValue = "test-var-secret-value"

	tlsSecretName = "test-tls-secret"
	ca            = "test-ca"
	cert          = "test-cert"
	key           = "test-key"
	caValue       = "test-ca-value"
	certValue     = "test-cert-value"
	keyValue      = "test-key-value"

	secretNamespace = "test-secret-namespace"

	clusterName = "test-cluster-name"
)

func TestMakeConfig(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()

	sut := NewFluentBitConfigBuilder(fakeClient)

	t.Run("file config", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithFile("test-file", "test-filecontent").
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)
		require.NoError(t, err)

		sectionsConfig, err := buildFluentBitSectionsConfig(&pipelines[0], sut.cfg, clusterName)
		require.NoError(t, err)

		expectedConfig := &FluentBitConfig{
			SectionsConfig:  map[string]string{"test-pipeline.conf": sectionsConfig},
			FilesConfig:     map[string]string{"test-file": "test-filecontent"},
			EnvConfigSecret: map[string][]byte{},
			TLSConfigSecret: map[string][]byte{},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})
}

func TestBuildEnvConfigSecret(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hostSecretName,
				Namespace: secretNamespace,
			},
			Data: map[string][]byte{
				hostSecretKey: []byte(hostSecretValue),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      basicAuthSecretName,
				Namespace: secretNamespace,
			},
			Data: map[string][]byte{
				basicAuthUserKey:     []byte(basicAuthUserValue),
				basicAuthPasswordKey: []byte(basicAuthPasswordValue),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varSecretName,
				Namespace: secretNamespace,
			},
			Data: map[string][]byte{
				varSecretKey: []byte(varSecretValue),
			},
		}).Build()
	sut := NewFluentBitConfigBuilder(fakeClient)

	t.Run("host from secret", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPHostFromSecret(hostSecretName, secretNamespace, hostSecretKey),
				).
				WithFile("test-file", "test-filecontent").
				WithVariable("test-var", varSecretName, secretNamespace, varSecretKey).
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)
		require.NoError(t, err)

		sectionsConfig, err := buildFluentBitSectionsConfig(&pipelines[0], sut.cfg, clusterName)
		require.NoError(t, err)

		envConfigSecretKey := formatEnvVarName("test-pipeline", secretNamespace, hostSecretName, hostSecretKey)

		expectedConfig := &FluentBitConfig{
			SectionsConfig: map[string]string{"test-pipeline.conf": sectionsConfig},
			FilesConfig:    map[string]string{"test-file": "test-filecontent"},
			EnvConfigSecret: map[string][]byte{
				envConfigSecretKey: []byte(hostSecretValue),
				"test-var":         []byte(varSecretValue),
			},
			TLSConfigSecret: map[string][]byte{},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})

	t.Run("basic auth from secret", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPBasicAuthFromSecret(basicAuthSecretName, secretNamespace, basicAuthUserKey, basicAuthPasswordKey),
				).
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)
		require.NoError(t, err)

		sectionsConfig, err := buildFluentBitSectionsConfig(&pipelines[0], sut.cfg, clusterName)
		require.NoError(t, err)

		envConfigUserSecretKey := formatEnvVarName("test-pipeline", secretNamespace, basicAuthSecretName, basicAuthUserKey)
		envConfigPasswordSecretKey := formatEnvVarName("test-pipeline", secretNamespace, basicAuthSecretName, basicAuthPasswordKey)

		expectedConfig := &FluentBitConfig{
			SectionsConfig: map[string]string{"test-pipeline.conf": sectionsConfig},
			EnvConfigSecret: map[string][]byte{
				envConfigUserSecretKey:     []byte(basicAuthUserValue),
				envConfigPasswordSecretKey: []byte(basicAuthPasswordValue),
			},
			FilesConfig:     map[string]string{},
			TLSConfigSecret: map[string][]byte{},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})

	t.Run("multiple log pipelines", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithFile("test-file", "test-filecontent").
				WithHTTPOutput(
					testutils.HTTPBasicAuthFromSecret(basicAuthSecretName, secretNamespace, basicAuthUserKey, basicAuthPasswordKey),
				).Build(),
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline-2").
				WithFile("test-file2", "test-filecontent2").
				WithHTTPOutput(
					testutils.HTTPHostFromSecret(hostSecretName, secretNamespace, hostSecretKey),
				).Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)
		require.NoError(t, err)

		sectionsConfig, err := buildFluentBitSectionsConfig(&pipelines[0], sut.cfg, clusterName)
		require.NoError(t, err)
		sectionsConfig2, err := buildFluentBitSectionsConfig(&pipelines[1], sut.cfg, clusterName)
		require.NoError(t, err)

		envConfigUserSecretKey := formatEnvVarName("test-pipeline", secretNamespace, basicAuthSecretName, basicAuthUserKey)
		envConfigPasswordSecretKey := formatEnvVarName("test-pipeline", secretNamespace, basicAuthSecretName, basicAuthPasswordKey)
		envConfigHostSecretKey := formatEnvVarName("test-pipeline-2", secretNamespace, hostSecretName, hostSecretKey)

		expectedConfig := &FluentBitConfig{
			SectionsConfig: map[string]string{
				"test-pipeline.conf":   sectionsConfig,
				"test-pipeline-2.conf": sectionsConfig2,
			},
			EnvConfigSecret: map[string][]byte{
				envConfigUserSecretKey:     []byte(basicAuthUserValue),
				envConfigPasswordSecretKey: []byte(basicAuthPasswordValue),
				envConfigHostSecretKey:     []byte(hostSecretValue),
			},
			FilesConfig: map[string]string{
				"test-file":  "test-filecontent",
				"test-file2": "test-filecontent2",
			},
			TLSConfigSecret: map[string][]byte{},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})

	t.Run("should return error when secret reference is not found", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPHostFromSecret("test-invalid-secret", "test-invalid-secret-namespace", "test-invalid-key"),
				).
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)

		require.ErrorContains(t, err, "unable to read secret 'test-invalid-secret' from namespace 'test-invalid-secret-namespace'")
		require.Nil(t, fluentBitConfig)
	})

	t.Run("should return error when key is not found in a valid host secret", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPHostFromSecret(hostSecretName, secretNamespace, "test-invalid-key"),
				).
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)

		require.ErrorContains(t, err, "unable to find key 'test-invalid-key' in secret 'test-host-secret' from namespace 'test-secret-namespace'")
		require.Nil(t, fluentBitConfig)
	})
}

func TestBuildTLSConfigSecret(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tlsSecretName,
				Namespace: secretNamespace,
			},
			Data: map[string][]byte{
				ca:   []byte(caValue),
				cert: []byte(certValue),
				key:  []byte(keyValue),
			},
		}).Build()
	sut := NewFluentBitConfigBuilder(fakeClient)

	t.Run("tls from string", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPClientTLSFromString(caValue, certValue, keyValue),
				).
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)
		require.NoError(t, err)

		sectionsConfig, err := buildFluentBitSectionsConfig(&pipelines[0], sut.cfg, clusterName)
		require.NoError(t, err)

		expectedConfig := &FluentBitConfig{
			SectionsConfig:  map[string]string{"test-pipeline.conf": sectionsConfig},
			EnvConfigSecret: map[string][]byte{},
			FilesConfig:     map[string]string{},
			TLSConfigSecret: map[string][]byte{
				"test-pipeline-ca.crt":   []byte(caValue),
				"test-pipeline-cert.crt": []byte(certValue),
				"test-pipeline-key.key":  []byte(keyValue),
			},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})

	t.Run("tls from secret ref", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPClientTLS(telemetryv1beta1.OutputTLS{
						CA: &telemetryv1beta1.ValueType{
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Key:       ca,
									Name:      tlsSecretName,
									Namespace: secretNamespace},
							}},
						Cert: &telemetryv1beta1.ValueType{
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Key:       cert,
									Name:      tlsSecretName,
									Namespace: secretNamespace},
							}},
						Key: &telemetryv1beta1.ValueType{
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Key:       key,
									Name:      tlsSecretName,
									Namespace: secretNamespace},
							}},
					}),
				).
				Build(),
		}
		fluentBitConfig, err := sut.Build(ctx, pipelines, clusterName)
		require.NoError(t, err)

		sectionsConfig, err := buildFluentBitSectionsConfig(&pipelines[0], sut.cfg, clusterName)
		require.NoError(t, err)

		expectedConfig := &FluentBitConfig{
			SectionsConfig:  map[string]string{"test-pipeline.conf": sectionsConfig},
			EnvConfigSecret: map[string][]byte{},
			FilesConfig:     map[string]string{},
			TLSConfigSecret: map[string][]byte{
				"test-pipeline-ca.crt":   []byte(caValue),
				"test-pipeline-cert.crt": []byte(certValue),
				"test-pipeline-key.key":  []byte(keyValue),
			},
		}

		require.Equal(t, expectedConfig, fluentBitConfig)
	})

	t.Run("should return error when key is not found in a valid tls secret", func(t *testing.T) {
		pipelines := []telemetryv1beta1.LogPipeline{
			testutils.NewLogPipelineBuilder().
				WithName("test-pipeline").
				WithHTTPOutput(
					testutils.HTTPClientTLS(telemetryv1beta1.OutputTLS{
						CA: &telemetryv1beta1.ValueType{
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Key:       "test-invalid-ca-value",
									Name:      tlsSecretName,
									Namespace: secretNamespace},
							}},
						Cert: &telemetryv1beta1.ValueType{
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Key:       cert,
									Name:      tlsSecretName,
									Namespace: secretNamespace},
							}},
						Key: &telemetryv1beta1.ValueType{
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Key:       key,
									Name:      tlsSecretName,
									Namespace: secretNamespace},
							}},
					}),
				).
				Build(),
		}
		fluentBitConfig, err := sut.Build(
			ctx,
			pipelines, clusterName)

		require.ErrorContains(t, err, "unable to build tls secret")
		require.Nil(t, fluentBitConfig)
	})
}
