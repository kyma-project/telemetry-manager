package logpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestGetDeployableLogPipelines(t *testing.T) {
	timestamp := metav1.Now()
	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.LogPipeline
		deployablePipelines bool
	}{
		{
			name: "should reject LogPipelines which are being deleted",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pipeline-in-deletion",
						DeletionTimestamp: &timestamp,
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "Name	stdout\n",
						}},
				},
			},
			deployablePipelines: false,
		},
		{
			name: "should reject LogPipelines with missing Secrets",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-secret",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							HTTP: &telemetryv1alpha1.HTTPOutput{
								Host: telemetryv1alpha1.ValueType{
									ValueFrom: &telemetryv1alpha1.ValueFromSource{
										SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
											Name:      "some-secret",
											Namespace: "some-namespace",
											Key:       "host",
										},
									},
								},
							},
						}},
				},
			},
			deployablePipelines: false,
		},
		{
			name: "should reject LogPipelines with Loki Output",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-loki-output",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Loki: &telemetryv1alpha1.LokiOutput{
								URL: telemetryv1alpha1.ValueType{
									Value: "http://logging-loki:3100/loki/api/v1/push",
								},
							},
						}},
				},
			},
			deployablePipelines: false,
		},
		{
			name: "should accept healthy LogPipelines",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-stdout-1",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "Name	stdout\n",
						}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-stdout-2",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "Name	stdout\n",
						}},
				},
			},
			deployablePipelines: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			deployablePipelines := getDeployableLogPipelines(ctx, test.pipelines, fakeClient)
			for _, pipeline := range test.pipelines {
				if test.deployablePipelines == true {
					require.Contains(t, deployablePipelines, pipeline)
				} else {
					require.NotContains(t, deployablePipelines, pipeline)
				}
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
		EnvSecret: types.NamespacedName{
			Namespace: "default",
			Name:      "env",
		},
		OutputTLSConfigSecret: types.NamespacedName{
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
			Name:      config.EnvSecret.Name,
			Namespace: config.EnvSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}
	certSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.OutputTLSConfigSecret.Name,
			Namespace: config.OutputTLSConfigSecret.Namespace,
		},
		Data: map[string][]byte{
			"a": []byte("b"),
		},
	}

	client := fake.NewClientBuilder().WithObjects(&dsConfig, &sectionsConfig, &filesConfig, &luaConfig, &parsersConfig, &envSecret, &certSecret).Build()
	r := Reconciler{
		Client: client,
		config: config,
	}
	ctx := context.Background()

	checksum, err := r.calculateChecksum(ctx)

	t.Run("Initial checksum should not be empty", func(t *testing.T) {
		require.NoError(t, err)
		require.NotEmpty(t, checksum)
	})

	t.Run("Changing static config should update checksum", func(t *testing.T) {
		dsConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &dsConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating static config")
		checksum = newChecksum
	})

	t.Run("Changing sections config should update checksum", func(t *testing.T) {
		sectionsConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &sectionsConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating sections config")
		checksum = newChecksum
	})

	t.Run("Changing files config should update checksum", func(t *testing.T) {
		filesConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &filesConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating files config")
		checksum = newChecksum
	})

	t.Run("Changing LUA config should update checksum", func(t *testing.T) {
		luaConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &luaConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating LUA config")
		checksum = newChecksum
	})

	t.Run("Changing parsers config should update checksum", func(t *testing.T) {
		parsersConfig.Data["a"] = "c"
		updateErr := client.Update(ctx, &parsersConfig)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating parsers config")
		checksum = newChecksum
	})

	t.Run("Changing env Secret should update checksum", func(t *testing.T) {
		envSecret.Data["a"] = []byte("c")
		updateErr := client.Update(ctx, &envSecret)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating env secret")
		checksum = newChecksum
	})

	t.Run("Changing certificate Secret should update checksum", func(t *testing.T) {
		certSecret.Data["a"] = []byte("c")
		updateErr := client.Update(ctx, &certSecret)
		require.NoError(t, updateErr)

		newChecksum, checksumErr := r.calculateChecksum(ctx)
		require.NoError(t, checksumErr)
		require.NotEqualf(t, checksum, newChecksum, "Checksum not changed by updating certificate secret")
	})
}
