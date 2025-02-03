package fluentbit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/utils/k8s/mocks"
)

func TestSyncSectionsConfigMap(t *testing.T) {
	sectionsCmName := types.NamespacedName{Name: "sections", Namespace: "telemetry-system"}
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sectionsCmName.Name,
				Namespace: sectionsCmName.Namespace,
			},
		}).Build()
	require.NoError(t, telemetryv1alpha1.AddToScheme(fakeClient.Scheme()))

	t.Run("should add section during first sync", func(t *testing.T) {
		sut := Syncer{fakeClient, Config{SectionsConfigMap: sectionsCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.LogPipelineOutput{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		var deployableLogPipeline []telemetryv1alpha1.LogPipeline
		deployableLogPipeline = append(deployableLogPipeline, *pipeline)
		err := sut.syncSectionsConfigMap(context.Background(), pipeline, deployableLogPipeline)
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.Contains(t, sectionsCm.Data, "noop.conf")
		require.Contains(t, sectionsCm.Data["noop.conf"], "foo")
		require.Len(t, sectionsCm.OwnerReferences, 1)
		require.Equal(t, pipeline.Name, sectionsCm.OwnerReferences[0].Name)
	})

	t.Run("should update section during subsequent sync", func(t *testing.T) {
		sut := Syncer{fakeClient, Config{SectionsConfigMap: sectionsCmName}}
		require.NoError(t, telemetryv1alpha1.AddToScheme(fakeClient.Scheme()))

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.LogPipelineOutput{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		var deployableLogPipeline []telemetryv1alpha1.LogPipeline
		deployableLogPipeline = append(deployableLogPipeline, *pipeline)

		err := sut.syncSectionsConfigMap(context.Background(), pipeline, deployableLogPipeline)
		require.NoError(t, err)

		pipeline.Spec.Output.Custom = `
name  null
alias bar`
		err = sut.syncSectionsConfigMap(context.Background(), pipeline, deployableLogPipeline)
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.Contains(t, sectionsCm.Data, "noop.conf")
		require.NotContains(t, sectionsCm.Data["noop.conf"], "foo")
		require.Contains(t, sectionsCm.Data["noop.conf"], "bar")
	})

	t.Run("should remove section if marked for deletion", func(t *testing.T) {
		sut := Syncer{fakeClient, Config{SectionsConfigMap: sectionsCmName}}
		require.NoError(t, telemetryv1alpha1.AddToScheme(fakeClient.Scheme()))

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.LogPipelineOutput{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		var deployableLogPipeline []telemetryv1alpha1.LogPipeline

		err := sut.syncSectionsConfigMap(context.Background(), pipeline, deployableLogPipeline)
		require.NoError(t, err)

		now := metav1.Now()
		pipeline.SetDeletionTimestamp(&now)
		err = sut.syncSectionsConfigMap(context.Background(), pipeline, deployableLogPipeline)
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.NotContains(t, sectionsCm.Data, "noop.conf")
	})

	t.Run("should fail if client fails", func(t *testing.T) {
		badReqClient := &mocks.Client{}
		badReqErr := apierrors.NewBadRequest("")
		badReqClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		badReqClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		sut := Syncer{badReqClient, Config{}}

		lp := telemetryv1alpha1.LogPipeline{}
		err := sut.syncFilesConfigMap(context.Background(), &lp)

		require.Error(t, err)
	})

	t.Run("should handle multiple pipelines and deletion properly", func(t *testing.T) {
		sut := Syncer{
			Client: fakeClient,
			Config: Config{
				SectionsConfigMap: sectionsCmName,
			},
		}

		// Create pipeline1 with HTTP output
		pipeline1 := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipeline1",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.LogPipelineOutput{
					HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
						Host: telemetryv1alpha1.ValueType{Value: "host"},
					},
				},
			},
		}

		// Create pipeline2 with HTTP output
		pipeline2 := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipeline2",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.LogPipelineOutput{
					HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
						Host: telemetryv1alpha1.ValueType{Value: "host"},
					},
				},
			},
		}

		// Test adding both pipelines
		deployablePipelines := []telemetryv1alpha1.LogPipeline{*pipeline1, *pipeline2}

		err := sut.syncSectionsConfigMap(context.Background(), pipeline1, deployablePipelines)
		require.NoError(t, err)
		err = sut.syncSectionsConfigMap(context.Background(), pipeline2, deployablePipelines)
		require.NoError(t, err)

		// Verify both pipelines are in configmap
		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &cm)
		require.NoError(t, err)
		require.Contains(t, cm.Data, "pipeline1.conf")
		require.Contains(t, cm.Data, "pipeline2.conf")
		require.Contains(t, cm.Data["pipeline1.conf"], "pipeline1")
		require.Contains(t, cm.Data["pipeline2.conf"], "pipeline2")

		// Mark pipeline1 for deletion
		now := metav1.Now()
		pipeline1.DeletionTimestamp = &now

		// Update configmap with pipeline1 deleted
		deployablePipelineAfterDeletion := []telemetryv1alpha1.LogPipeline{*pipeline2}
		err = sut.syncSectionsConfigMap(context.Background(), pipeline1, deployablePipelineAfterDeletion)
		require.NoError(t, err)

		// Verify pipeline1 is removed but pipeline2 remains
		err = fakeClient.Get(context.Background(), sectionsCmName, &cm)
		require.NoError(t, err)
		require.NotContains(t, cm.Data, "pipeline1.conf")
		require.Contains(t, cm.Data, "pipeline2.conf")
		require.Contains(t, cm.Data["pipeline2.conf"], "pipeline2")
	})
}

func TestSyncFilesConfigMap(t *testing.T) {
	filesCmName := types.NamespacedName{Name: "files", Namespace: "telemetry-system"}
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      filesCmName.Name,
				Namespace: filesCmName.Namespace,
			},
		}).Build()

	t.Run("should add files during first sync", func(t *testing.T) {
		sut := Syncer{fakeClient, Config{FilesConfigMap: filesCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Files: []telemetryv1alpha1.LogPipelineFileMount{
					{Name: "lua-script", Content: "here comes some lua code"},
					{Name: "js-script", Content: "here comes some js code"},
				},
				Output: telemetryv1alpha1.LogPipelineOutput{
					Custom: `
name  null
alias foo`,
				},
			},
		}
		err := sut.syncFilesConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		var filesCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), filesCmName, &filesCm)
		require.NoError(t, err)
		require.Contains(t, filesCm.Data, "lua-script")
		require.Contains(t, filesCm.Data["lua-script"], "here comes some lua code")
		require.Contains(t, filesCm.Data, "js-script")
		require.Contains(t, filesCm.Data["js-script"], "here comes some js code")
		require.Len(t, filesCm.OwnerReferences, 1)
		require.Equal(t, pipeline.Name, filesCm.OwnerReferences[0].Name)
	})

	t.Run("should update files during subsequent sync", func(t *testing.T) {
		sut := Syncer{fakeClient, Config{FilesConfigMap: filesCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Files: []telemetryv1alpha1.LogPipelineFileMount{
					{Name: "lua-script", Content: "here comes some lua code"},
				},
				Output: telemetryv1alpha1.LogPipelineOutput{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		err := sut.syncFilesConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		pipeline.Spec.Files[0].Content = "here comes some more lua code"
		err = sut.syncFilesConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		var filesCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), filesCmName, &filesCm)
		require.NoError(t, err)
		require.Contains(t, filesCm.Data, "lua-script")
		require.Contains(t, filesCm.Data["lua-script"], "here comes some more lua code")
	})

	t.Run("should remove files if marked for deletion", func(t *testing.T) {
		sut := Syncer{fakeClient, Config{FilesConfigMap: filesCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Files: []telemetryv1alpha1.LogPipelineFileMount{
					{Name: "lua-script", Content: "here comes some lua code"},
				},
				Output: telemetryv1alpha1.LogPipelineOutput{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		err := sut.syncFilesConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		now := metav1.Now()
		pipeline.SetDeletionTimestamp(&now)
		err = sut.syncFilesConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		var filesCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), filesCmName, &filesCm)
		require.NoError(t, err)
		require.NotContains(t, filesCm.Data, "lua-script")
	})

	t.Run("should fail if client fails", func(t *testing.T) {
		badReqClient := &mocks.Client{}
		badReqErr := apierrors.NewBadRequest("")
		badReqClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		badReqClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		sut := Syncer{badReqClient, Config{}}

		lp := telemetryv1alpha1.LogPipeline{}
		err := sut.syncFilesConfigMap(context.Background(), &lp)

		require.Error(t, err)
	})
}

func TestSyncEnvSecret(t *testing.T) {
	allPipelines := telemetryv1alpha1.LogPipelineList{
		Items: []telemetryv1alpha1.LogPipeline{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "http"},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{Value: "localhost"},
							User: telemetryv1alpha1.ValueType{Value: "admin"},
							Password: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "creds",
										Namespace: "default",
										Key:       "password",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("should add value to env secret during first sync", func(t *testing.T) {
		credsSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "creds",
				Namespace: "default",
			},
			Data: map[string][]byte{"password": []byte("qwerty")},
		}
		fakeClient := fake.NewClientBuilder().WithObjects(&credsSecret).Build()

		envSecretName := types.NamespacedName{Name: "env", Namespace: "telemetry-system"}
		sut := Syncer{fakeClient, Config{EnvConfigSecret: envSecretName}}
		err := sut.syncEnvConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(context.Background(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.Contains(t, envSecret.Data, "HTTP_DEFAULT_CREDS_PASSWORD")
		require.Equal(t, []byte("qwerty"), envSecret.Data["HTTP_DEFAULT_CREDS_PASSWORD"])
		require.Len(t, envSecret.OwnerReferences, 1)
		require.Equal(t, allPipelines.Items[0].Name, envSecret.OwnerReferences[0].Name)
	})

	t.Run("should update value in env secret during subsequent sync", func(t *testing.T) {
		passwordSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "creds",
				Namespace: "default",
			},
			Data: map[string][]byte{"password": []byte("qwerty")},
		}
		fakeClient := fake.NewClientBuilder().WithObjects(&passwordSecret).Build()

		envSecretName := types.NamespacedName{Name: "env", Namespace: "telemetry-system"}
		sut := Syncer{fakeClient, Config{EnvConfigSecret: envSecretName}}
		err := sut.syncEnvConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		passwordSecret.Data["password"] = []byte("qwertz")
		err = fakeClient.Update(context.Background(), &passwordSecret)
		require.NoError(t, err)

		err = sut.syncEnvConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(context.Background(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.Contains(t, envSecret.Data, "HTTP_DEFAULT_CREDS_PASSWORD")
		require.Equal(t, []byte("qwertz"), envSecret.Data["HTTP_DEFAULT_CREDS_PASSWORD"])
	})

	t.Run("should delete value in env secret if marked for deletion", func(t *testing.T) {
		passwordSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "creds",
				Namespace: "default",
			},
			Data: map[string][]byte{"password": []byte("qwerty")},
		}
		fakeClient := fake.NewClientBuilder().WithObjects(&passwordSecret).Build()

		envSecretName := types.NamespacedName{Name: "env", Namespace: "telemetry-system"}
		sut := Syncer{fakeClient, Config{EnvConfigSecret: envSecretName}}
		err := sut.syncEnvConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		now := metav1.Now()
		allPipelines.Items[0].SetDeletionTimestamp(&now)
		err = sut.syncEnvConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(context.Background(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.NotContains(t, envSecret.Data, "HTTP_DEFAULT_CREDS_PASSWORD")
	})
}

func TestSyncTLSConfigSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	allPipelines := telemetryv1alpha1.LogPipelineList{
		Items: []telemetryv1alpha1.LogPipeline{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline-1"},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{Value: "localhost"},
							TLS: telemetryv1alpha1.LogPipelineOutputTLS{
								Disabled:                  false,
								SkipCertificateValidation: false,
								CA: &telemetryv1alpha1.ValueType{
									Value: "fake-ca-value",
								},
								Cert: &telemetryv1alpha1.ValueType{
									Value: "fake-cert-value",
								},
								Key: &telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "my-key-secret",
											Namespace: "default",
											Key:       "my-key.key",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("should add output TLS config to secret during first sync", func(t *testing.T) {
		keySecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-key-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{"my-key.key": []byte("fake-key-value")},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&keySecret).Build()

		config := Config{
			TLSFileConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
		}
		sut := Syncer{fakeClient, config}
		err := sut.syncTLSFileConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		var tlsConfigSecret corev1.Secret
		err = fakeClient.Get(context.Background(), config.TLSFileConfigSecret, &tlsConfigSecret)
		require.NoError(t, err)
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-ca.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-cert.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-key.key")
		require.Equal(t, []byte("fake-ca-value"), tlsConfigSecret.Data["pipeline-1-ca.crt"])
		require.Equal(t, []byte("fake-cert-value"), tlsConfigSecret.Data["pipeline-1-cert.crt"])
		require.Equal(t, []byte("fake-key-value"), tlsConfigSecret.Data["pipeline-1-key.key"])
		require.Len(t, tlsConfigSecret.OwnerReferences, 1)
		require.Equal(t, allPipelines.Items[0].Name, tlsConfigSecret.OwnerReferences[0].Name)
	})

	t.Run("should update output TLS config in secret during subsequent sync", func(t *testing.T) {
		keySecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-key-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{"my-key.key": []byte("fake-key-value")},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&keySecret).Build()

		config := Config{
			TLSFileConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
		}
		sut := Syncer{fakeClient, config}
		err := sut.syncTLSFileConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		keySecret.Data["my-key.key"] = []byte("new-fake-key-value")
		err = fakeClient.Update(context.Background(), &keySecret)
		require.NoError(t, err)

		err = sut.syncTLSFileConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		var tlsConfigSecret corev1.Secret
		err = fakeClient.Get(context.Background(), config.TLSFileConfigSecret, &tlsConfigSecret)
		require.NoError(t, err)
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-ca.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-cert.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-key.key")
		require.Equal(t, []byte("fake-ca-value"), tlsConfigSecret.Data["pipeline-1-ca.crt"])
		require.Equal(t, []byte("fake-cert-value"), tlsConfigSecret.Data["pipeline-1-cert.crt"])
		require.Equal(t, []byte("new-fake-key-value"), tlsConfigSecret.Data["pipeline-1-key.key"])
	})

	t.Run("should delete value in output TLS config secret if marked for deletion", func(t *testing.T) {
		keySecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-key-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{"my-key.key": []byte("fake-key-value")},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&keySecret).Build()

		config := Config{
			TLSFileConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
		}
		sut := Syncer{fakeClient, config}
		err := sut.syncTLSFileConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		now := metav1.Now()
		allPipelines.Items[0].SetDeletionTimestamp(&now)
		err = sut.syncTLSFileConfigSecret(context.Background(), allPipelines.Items)
		require.NoError(t, err)

		var tlsConfigSecret corev1.Secret
		err = fakeClient.Get(context.Background(), config.TLSFileConfigSecret, &tlsConfigSecret)
		require.NoError(t, err)
		require.NotContains(t, tlsConfigSecret.Data, "pipeline-1-ca.crt")
		require.NotContains(t, tlsConfigSecret.Data, "pipeline-1-cert.crt")
		require.NotContains(t, tlsConfigSecret.Data, "pipeline-1-key.key")
	})
}
