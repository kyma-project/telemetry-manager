package fluentbit

import (
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
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
	sectionsCmName := types.NamespacedName{Name: fbSectionsConfigMapName, Namespace: "telemetry-system"}
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sectionsCmName.Name,
				Namespace: sectionsCmName.Namespace,
			},
		}).Build()
	require.NoError(t, telemetryv1alpha1.AddToScheme(fakeClient.Scheme()))

	t.Run("should add section during first sync", func(t *testing.T) {
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			SectionsConfig: builder.SectionsConfig{
				Key:   "pipeline1.conf",
				Value: "test-value",
			},
		}, "telemetry-system"}

		err := sut.syncSectionsConfigMap(t.Context())
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(t.Context(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.Contains(t, sectionsCm.Data, "pipeline1.conf")
		require.Contains(t, sectionsCm.Data["pipeline1.conf"], "test-value")
	})

	t.Run("should update section during subsequent sync", func(t *testing.T) {
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			SectionsConfig: builder.SectionsConfig{
				Key:   "pipeline1.conf",
				Value: "test-value",
			},
		}, "telemetry-system"}

		require.NoError(t, telemetryv1alpha1.AddToScheme(fakeClient.Scheme()))

		err := sut.syncSectionsConfigMap(t.Context())
		require.NoError(t, err)
		sut.Config.SectionsConfig = builder.SectionsConfig{
			Key:   "pipeline1.conf",
			Value: "new-value",
		}

		err = sut.syncSectionsConfigMap(t.Context())
		require.NoError(t, err)

		var sectionsCm corev1.ConfigMap
		err = fakeClient.Get(t.Context(), sectionsCmName, &sectionsCm)
		require.NoError(t, err)
		require.Contains(t, sectionsCm.Data, "pipeline1.conf")
		require.NotContains(t, sectionsCm.Data["pipeline1.conf"], "test-value")
		require.Contains(t, sectionsCm.Data["pipeline1.conf"], "new-value")
	})

	t.Run("should fail if client fails", func(t *testing.T) {
		badReqClient := &mocks.Client{}
		badReqErr := apierrors.NewBadRequest("")
		badReqClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		badReqClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		sut := syncer{badReqClient, &builder.FluentBitConfig{}, ""}

		err := sut.syncSectionsConfigMap(t.Context())

		require.Error(t, err)
	})
}

func TestSyncFilesConfigMap(t *testing.T) {
	filesCmName := types.NamespacedName{Name: fbFilesConfigMapName, Namespace: "telemetry-system"}
	fakeClient := fake.NewClientBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      filesCmName.Name,
				Namespace: filesCmName.Namespace,
			},
		}).Build()

	t.Run("should add files during first sync", func(t *testing.T) {
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			FilesConfig: map[string]string{
				"lua-script": "here comes some lua code",
				"js-script":  "here comes some js code",
			},
		}, "telemetry-system"}

		err := sut.syncFilesConfigMap(t.Context())
		require.NoError(t, err)

		var filesCm corev1.ConfigMap
		err = fakeClient.Get(t.Context(), filesCmName, &filesCm)
		require.NoError(t, err)
		require.Contains(t, filesCm.Data, "lua-script")
		require.Contains(t, filesCm.Data["lua-script"], "here comes some lua code")
		require.Contains(t, filesCm.Data, "js-script")
		require.Contains(t, filesCm.Data["js-script"], "here comes some js code")
	})

	t.Run("should update files during subsequent sync", func(t *testing.T) {
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			FilesConfig: map[string]string{
				"lua-script": "here comes some lua code",
				"js-script":  "here comes some js code",
			},
		}, "telemetry-system"}

		err := sut.syncFilesConfigMap(t.Context())
		require.NoError(t, err)

		sut.Config.FilesConfig["lua-script"] = "here comes some new lua code"
		err = sut.syncFilesConfigMap(t.Context())
		require.NoError(t, err)

		var filesCm corev1.ConfigMap
		err = fakeClient.Get(t.Context(), filesCmName, &filesCm)
		require.NoError(t, err)
		require.Contains(t, filesCm.Data, "lua-script")
		require.Contains(t, filesCm.Data["lua-script"], "here comes some new lua code")
	})

	t.Run("should fail if client fails", func(t *testing.T) {
		badReqClient := &mocks.Client{}
		badReqErr := apierrors.NewBadRequest("")
		badReqClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		badReqClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
		sut := syncer{badReqClient, &builder.FluentBitConfig{}, "telemetry-system"}

		err := sut.syncFilesConfigMap(t.Context())

		require.Error(t, err)
	})
}

func TestSyncEnvSecret(t *testing.T) {
	t.Run("should add value to env secret during first sync", func(t *testing.T) {

		fakeClient := fake.NewClientBuilder().Build()

		envSecretName := types.NamespacedName{Name: fbEnvConfigSecretName, Namespace: "telemetry-system"}
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			EnvConfigSecret: map[string][]byte{
				"pipeline1-host": []byte("test-env-secret"),
			}}, "telemetry-system"}
		err := sut.syncEnvConfigSecret(t.Context())
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(t.Context(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.Contains(t, envSecret.Data, "pipeline1-host")
		require.Equal(t, []byte("test-env-secret"), envSecret.Data["pipeline1-host"])
	})

	t.Run("should update value in env secret during subsequent sync", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().Build()

		envSecretName := types.NamespacedName{Name: fbEnvConfigSecretName, Namespace: "telemetry-system"}
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			EnvConfigSecret: map[string][]byte{
				"pipeline1-password": []byte("test-env-secret"),
			},
		}, "telemetry-system"}
		err := sut.syncEnvConfigSecret(t.Context())
		require.NoError(t, err)
		sut.Config.EnvConfigSecret["pipeline1-host"] = []byte("test-env-secret-new")

		err = sut.syncEnvConfigSecret(t.Context())
		require.NoError(t, err)

		var envSecret corev1.Secret
		err = fakeClient.Get(t.Context(), envSecretName, &envSecret)
		require.NoError(t, err)
		require.Contains(t, envSecret.Data, "pipeline1-password")
		require.Equal(t, []byte("test-env-secret-new"), envSecret.Data["pipeline1-host"])
	})
}

func TestSyncTLSConfigSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	t.Run("should add output TLS config to secret during first sync", func(t *testing.T) {

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		tlsFileConfigSecretName := types.NamespacedName{Name: fbTLSFileConfigSecretName, Namespace: "default"}
		sut := syncer{fakeClient, &builder.FluentBitConfig{
			TLSConfigSecret: map[string][]byte{
				"pipeline-1-ca.crt":   []byte("fake-ca-value"),
				"pipeline-1-cert.crt": []byte("fake-cert-value"),
				"pipeline-1-key.key":  []byte("fake-key-value"),
			},
		}, "default"}
		err := sut.syncTLSFileConfigSecret(t.Context())
		require.NoError(t, err)

		var tlsConfigSecret corev1.Secret
		err = fakeClient.Get(t.Context(), tlsFileConfigSecretName, &tlsConfigSecret)
		require.NoError(t, err)
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-ca.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-cert.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-key.key")
		require.Equal(t, []byte("fake-ca-value"), tlsConfigSecret.Data["pipeline-1-ca.crt"])
		require.Equal(t, []byte("fake-cert-value"), tlsConfigSecret.Data["pipeline-1-cert.crt"])
		require.Equal(t, []byte("fake-key-value"), tlsConfigSecret.Data["pipeline-1-key.key"])
	})

	t.Run("should update output TLS config in secret during subsequent sync", func(t *testing.T) {

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		tlsFileConfigSecretName := types.NamespacedName{Name: fbTLSFileConfigSecretName, Namespace: "default"}

		sut := syncer{fakeClient, &builder.FluentBitConfig{
			TLSConfigSecret: map[string][]byte{
				"pipeline-1-ca.crt":   []byte("fake-ca-value"),
				"pipeline-1-cert.crt": []byte("fake-cert-value"),
				"pipeline-1-key.key":  []byte("fake-key-value"),
			},
		}, "default"}
		err := sut.syncTLSFileConfigSecret(t.Context())
		require.NoError(t, err)

		sut.Config.TLSConfigSecret["my-key.key"] = []byte("new-fake-key-value")

		err = sut.syncTLSFileConfigSecret(t.Context())
		require.NoError(t, err)

		var tlsConfigSecret corev1.Secret
		err = fakeClient.Get(t.Context(), tlsFileConfigSecretName, &tlsConfigSecret)
		require.NoError(t, err)
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-ca.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-cert.crt")
		require.Contains(t, tlsConfigSecret.Data, "pipeline-1-key.key")
		require.Equal(t, []byte("fake-ca-value"), tlsConfigSecret.Data["pipeline-1-ca.crt"])
		require.Equal(t, []byte("fake-cert-value"), tlsConfigSecret.Data["pipeline-1-cert.crt"])
		require.Equal(t, []byte("new-fake-key-value"), tlsConfigSecret.Data["my-key.key"])
	})
}
