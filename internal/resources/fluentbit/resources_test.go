package fluentbit

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestAgent_ApplyResources(t *testing.T) {
	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	priorityClassName := "foo-prio-class"
	logPipeline := testutils.NewLogPipelineBuilder().WithName("foo-logpipeline").Build()

	tests := []struct {
		name           string
		sut            *AgentApplierDeleter
		goldenFilePath string
		saveGoldenFile bool
	}{
		{
			name:           "fluentbit",
			sut:            NewFluentBitApplierDeleter(image, exporterImage, priorityClassName),
			goldenFilePath: "testdata/fluentbit.yaml",
		},
	}

	for _, tt := range tests {
		var objects []client.Object

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
		utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
				objects = append(objects, obj)
				// Nothing has to be created, just add created object to the list
				return nil
			},
			// Update interceptor is needed for syncSectionsConfigMap operation
			Update: func(_ context.Context, c client.WithWatch, obj client.Object, option ...client.UpdateOption) error {
				// For updates, we'll either update the existing object in our slice
				// or append it if it doesn't exist
				found := false
				for i, existingObj := range objects {
					if existingObj.GetName() == obj.GetName() && existingObj.GetNamespace() == obj.GetNamespace() {
						objects[i] = obj
						found = true
						break
					}
				}
				if !found {
					objects = append(objects, obj)
				}
				return nil
			},
			Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				// Simulate that the object exists but is empty
				// This is needed for GetOrCreate operations
				return nil
			},
		}).Build()

		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(context.Background(), fakeClient, AgentApplyOptions{
				Config: Config{
					DaemonSet:           types.NamespacedName{Name: "foo-daemonset", Namespace: "kyma-system"},
					SectionsConfigMap:   types.NamespacedName{Name: "foo-sectionscm", Namespace: "kyma-system"},
					FilesConfigMap:      types.NamespacedName{Name: "foo-filescm", Namespace: "kyma-system"},
					LuaConfigMap:        types.NamespacedName{Name: "foo-luacm", Namespace: "kyma-system"},
					ParsersConfigMap:    types.NamespacedName{Name: "foo-parserscm", Namespace: "kyma-system"},
					EnvConfigSecret:     types.NamespacedName{Name: "foo-evnconfigsecret", Namespace: "kyma-system"},
					TLSFileConfigSecret: types.NamespacedName{Name: "foo-tlsfileconfigsecret", Namespace: "kyma-system"},
				},
				AllowedPorts: []int32{5555, 6666},

				Pipeline:               &logPipeline,
				DeployableLogPipelines: []telemetryv1alpha1.LogPipeline{logPipeline},
			})
			require.NoError(t, err)

			if tt.saveGoldenFile {
				testutils.SaveAsYAML(t, scheme, objects, tt.goldenFilePath)
			}

			bytes, err := testutils.MarshalYAML(scheme, objects)
			require.NoError(t, err)

			goldenFileBytes, err := os.ReadFile(tt.goldenFilePath)
			require.NoError(t, err)

			require.Equal(t, string(goldenFileBytes), string(bytes))
		})
	}
}

func TestAgent_DeleteResources(t *testing.T) {
	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	priorityClassName := "foo-prio-class"
	logPipeline := testutils.NewLogPipelineBuilder().WithName("foo-logpipeline").Build()

	var created []client.Object

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			created = append(created, obj)
			return c.Create(ctx, obj)
		},
	}).Build()

	tests := []struct {
		name string
		sut  *AgentApplierDeleter
	}{
		{
			name: "fluentbit",
			sut:  NewFluentBitApplierDeleter(image, exporterImage, priorityClassName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentApplyOptions := AgentApplyOptions{
				Config: Config{
					DaemonSet:           types.NamespacedName{Name: "foo-daemonset", Namespace: "kyma-system"},
					SectionsConfigMap:   types.NamespacedName{Name: "foo-sectionscm", Namespace: "kyma-system"},
					FilesConfigMap:      types.NamespacedName{Name: "foo-filescm", Namespace: "kyma-system"},
					LuaConfigMap:        types.NamespacedName{Name: "foo-luacm", Namespace: "kyma-system"},
					ParsersConfigMap:    types.NamespacedName{Name: "foo-parserscm", Namespace: "kyma-system"},
					EnvConfigSecret:     types.NamespacedName{Name: "foo-evnconfigsecret", Namespace: "kyma-system"},
					TLSFileConfigSecret: types.NamespacedName{Name: "foo-tlsfileconfigsecret", Namespace: "kyma-system"},
				},
				AllowedPorts:           []int32{5555, 6666},
				Pipeline:               &logPipeline,
				DeployableLogPipelines: []telemetryv1alpha1.LogPipeline{logPipeline},
			}

			err := tt.sut.ApplyResources(context.Background(), fakeClient, agentApplyOptions)
			require.NoError(t, err)

			err = tt.sut.DeleteResources(context.Background(), fakeClient, agentApplyOptions)
			require.NoError(t, err)

			for i := range created {
				// an update operation on a non-existent object should return a NotFound error
				err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(created[i]), created[i])
				require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
			}
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	config := Config{
		DaemonSet: types.NamespacedName{
			Namespace: "default",
			Name:      "daemonset",
		},
		SectionsConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "sections",
		},
		FilesConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "files",
		},
		LuaConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "lua",
		},
		ParsersConfigMap: types.NamespacedName{
			Namespace: "default",
			Name:      "parsers",
		},
		EnvConfigSecret: types.NamespacedName{
			Namespace: "default",
			Name:      "env",
		},
		TLSFileConfigSecret: types.NamespacedName{
			Namespace: "default",
			Name:      "tls",
		},
	}
	dsConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.DaemonSet.Name,
			Namespace: config.DaemonSet.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	sectionsConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SectionsConfigMap.Name,
			Namespace: config.SectionsConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	filesConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.FilesConfigMap.Name,
			Namespace: config.FilesConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	luaConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.LuaConfigMap.Name,
			Namespace: config.LuaConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	parsersConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ParsersConfigMap.Name,
			Namespace: config.ParsersConfigMap.Namespace,
		},
		Data: map[string]string{
			"a": "b",
		},
	}
	envSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EnvConfigSecret.Name,
			Namespace: config.EnvConfigSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}
	certSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.TLSFileConfigSecret.Name,
			Namespace: config.TLSFileConfigSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}

	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	priorityClassName := "foo-prio-class"

	aad := NewFluentBitApplierDeleter(image, exporterImage, priorityClassName)

	opts := AgentApplyOptions{
		Config: config,
	}

	client := fake.NewClientBuilder().WithObjects(&dsConfig, &sectionsConfig, &filesConfig, &luaConfig, &parsersConfig, &envSecret, &certSecret).Build()

	ctx := context.Background()

	checksum, err := aad.calculateChecksum(ctx, client, opts)

	t.Run("Initial checksum should not be empty", func(t *testing.T) {
		require.NoError(t, err)
		require.NotEmpty(t, checksum)
	})

	t.Run("Changing static config should update checksum", func(t *testing.T) {
		dsConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &dsConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating static config")
		checksum = newChecksum
	})

	t.Run("Changing sections config should update checksum", func(t *testing.T) {
		sectionsConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &sectionsConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating sections config")
		checksum = newChecksum
	})

	t.Run("Changing files config should update checksum", func(t *testing.T) {
		filesConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &filesConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating files config")
		checksum = newChecksum
	})

	t.Run("Changing LUA config should update checksum", func(t *testing.T) {
		luaConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &luaConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating LUA config")
		checksum = newChecksum
	})

	t.Run("Changing parsers config should update checksum", func(t *testing.T) {
		parsersConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &parsersConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating parsers config")
		checksum = newChecksum
	})

	t.Run("Changing env Secret should update checksum", func(t *testing.T) {
		envSecret.Data["a"] = []byte("c")
		updateErr := client.Update(ctx, &envSecret)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating env secret")
		checksum = newChecksum
	})

	t.Run("Changing certificate Secret should update checksum", func(t *testing.T) {
		certSecret.Data["a"] = []byte("c")
		updateErr := client.Update(ctx, &certSecret)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := aad.calculateChecksum(ctx, client, opts)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating certificate secret")
	})
}
