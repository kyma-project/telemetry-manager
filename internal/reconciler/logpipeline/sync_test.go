package logpipeline

import (
	"context"
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes/mocks"
	resources "github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

var (
	testConfig = Config{
		DaemonSet:         types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		SectionsConfigMap: types.NamespacedName{Name: "test-telemetry-fluent-bit-sections", Namespace: "default"},
		FilesConfigMap:    types.NamespacedName{Name: "test-telemetry-fluent-bit-files", Namespace: "default"},
		EnvSecret:         types.NamespacedName{Name: "test-telemetry-fluent-bit-env", Namespace: "default"},
		OverrideConfigMap: types.NamespacedName{Name: "override-config", Namespace: "default"},
		DaemonSetConfig: resources.DaemonSetConfig{
			FluentBitImage:              "my-fluent-bit-image",
			FluentBitConfigPrepperImage: "my-fluent-bit-config-image",
			ExporterImage:               "my-exporter-image",
			PriorityClassName:           "my-priority-class",
			CPULimit:                    resource.MustParse("1"),
			MemoryLimit:                 resource.MustParse("500Mi"),
			CPURequest:                  resource.MustParse(".1"),
			MemoryRequest:               resource.MustParse("100Mi"),
		},
		PipelineDefaults: builder.PipelineDefaults{
			InputTag:          "kube",
			MemoryBufferLimit: "10M",
			StorageType:       "filesystem",
			FsBufferLimit:     "1G",
		},
	}
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

	t.Run("should add section during first sync", func(t *testing.T) {
		sut := syncer{fakeClient, Config{SectionsConfigMap: sectionsCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					Custom: `
name  null
alias foo`,
				},
			},
		}
		err := sut.syncSectionsConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.Contains(t, sectionsCm.Data, "noop.conf")
		require.Contains(t, sectionsCm.Data["noop.conf"], "foo")
	})

	t.Run("should update section during subsequent sync", func(t *testing.T) {
		sut := syncer{fakeClient, Config{SectionsConfigMap: sectionsCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		err := sut.syncSectionsConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		pipeline.Spec.Output.Custom = `
name  null
alias bar`
		err = sut.syncSectionsConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.Contains(t, sectionsCm.Data, "noop.conf")
		require.NotContains(t, sectionsCm.Data["noop.conf"], "foo")
		require.Contains(t, sectionsCm.Data["noop.conf"], "bar")
	})

	t.Run("should remove section if marked for deletion", func(t *testing.T) {
		sut := syncer{fakeClient, Config{SectionsConfigMap: sectionsCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					Custom: `
name  null
alias foo`,
				},
			},
		}

		err := sut.syncSectionsConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		now := metav1.Now()
		pipeline.SetDeletionTimestamp(&now)
		err = sut.syncSectionsConfigMap(context.Background(), pipeline)
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.NotContains(t, sectionsCm.Data, "noop.conf")
	})

	t.Run("should fail if client fails", func(t *testing.T) {
		badReqClient := &mocks.Client{}
		badReqErr := errors.NewBadRequest("")
		badReqClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		badReqClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		sut := syncer{badReqClient, testConfig}

		lp := telemetryv1alpha1.LogPipeline{}
		err := sut.syncFilesConfigMap(context.Background(), &lp)

		require.Error(t, err)
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
		sut := syncer{fakeClient, Config{FilesConfigMap: filesCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Files: []telemetryv1alpha1.FileMount{
					{Name: "lua-script", Content: "here comes some lua code"},
					{Name: "js-script", Content: "here comes some js code"},
				},
				Output: telemetryv1alpha1.Output{
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
	})

	t.Run("should update files during subsequent sync", func(t *testing.T) {
		sut := syncer{fakeClient, Config{FilesConfigMap: filesCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Files: []telemetryv1alpha1.FileMount{
					{Name: "lua-script", Content: "here comes some lua code"},
				},
				Output: telemetryv1alpha1.Output{
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
		sut := syncer{fakeClient, Config{FilesConfigMap: filesCmName}}

		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "noop",
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Files: []telemetryv1alpha1.FileMount{
					{Name: "lua-script", Content: "here comes some lua code"},
				},
				Output: telemetryv1alpha1.Output{
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
		badReqErr := errors.NewBadRequest("")
		badReqClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		badReqClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		sut := syncer{badReqClient, testConfig}

		lp := telemetryv1alpha1.LogPipeline{}
		err := sut.syncFilesConfigMap(context.Background(), &lp)

		require.Error(t, err)
	})
}

func TestSyncReferencedSecrets(t *testing.T) {
	allPipelines := telemetryv1alpha1.LogPipelineList{
		Items: []telemetryv1alpha1.LogPipeline{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "http"},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.Output{
						HTTP: &telemetryv1alpha1.HTTPOutput{
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
		sut := syncer{fakeClient, Config{EnvSecret: envSecretName}}
		err := sut.syncReferencedSecrets(context.Background(), &allPipelines)
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(context.Background(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.Contains(t, envSecret.Data, "HTTP_DEFAULT_CREDS_PASSWORD")
		require.Equal(t, []byte("qwerty"), envSecret.Data["HTTP_DEFAULT_CREDS_PASSWORD"])
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
		sut := syncer{fakeClient, Config{EnvSecret: envSecretName}}
		err := sut.syncReferencedSecrets(context.Background(), &allPipelines)
		require.NoError(t, err)

		passwordSecret.Data["password"] = []byte("qwertz")
		err = fakeClient.Update(context.Background(), &passwordSecret)
		require.NoError(t, err)

		err = sut.syncReferencedSecrets(context.Background(), &allPipelines)
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
		sut := syncer{fakeClient, Config{EnvSecret: envSecretName}}
		err := sut.syncReferencedSecrets(context.Background(), &allPipelines)
		require.NoError(t, err)

		now := metav1.Now()
		allPipelines.Items[0].SetDeletionTimestamp(&now)
		err = sut.syncReferencedSecrets(context.Background(), &allPipelines)
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(context.Background(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.NotContains(t, envSecret.Data, "HTTP_DEFAULT_CREDS_PASSWORD")
	})
}
