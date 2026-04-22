package coordinationconfig

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/pipelines"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

// getPipelineRefs returns the pipeline references from config for the given signal type.
func getPipelineRefs(config *OTLPGatewayConfigMap, signalType pipelines.SignalType) []PipelineReference {
	switch signalType { //nolint:exhaustive // no gateway for FluentBit
	case pipelines.SignalTypeTrace:
		return config.TracePipelineReferences
	case pipelines.SignalTypeLog:
		return config.LogPipelineReferences
	case pipelines.SignalTypeMetric:
		return config.MetricPipelineReferences
	default:
		return nil
	}
}

// yamlKeyForSignalType returns the YAML key used in the ConfigMap for the given signal type.
func yamlKeyForSignalType(signalType pipelines.SignalType) string {
	switch signalType { //nolint:exhaustive // no gateway for FluentBit
	case pipelines.SignalTypeTrace:
		return "tracePipelines"
	case pipelines.SignalTypeLog:
		return "logPipelines"
	case pipelines.SignalTypeMetric:
		return "metricPipelines"
	default:
		return ""
	}
}

var allSignalTypes = []pipelines.SignalType{
	pipelines.SignalTypeTrace,
	pipelines.SignalTypeLog,
	pipelines.SignalTypeMetric,
}

func TestReadOTLPGatewayConfig_ConfigMapNotExist(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.TracePipelineReferences)
	require.Empty(t, config.LogPipelineReferences)
	require.Empty(t, config.MetricPipelineReferences)
}

func TestReadOTLPGatewayConfig_EmptyConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.TracePipelineReferences)
	require.Empty(t, config.LogPipelineReferences)
	require.Empty(t, config.MetricPipelineReferences)
}

func TestReadOTLPGatewayConfig_WithPipelines(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `tracePipelines:
- name: pipeline-1
  generation: 5
- name: pipeline-2
  generation: 10
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Len(t, config.TracePipelineReferences, 2)
	require.Equal(t, "pipeline-1", config.TracePipelineReferences[0].Name)
	require.Equal(t, int64(5), config.TracePipelineReferences[0].Generation)
	require.Equal(t, "pipeline-2", config.TracePipelineReferences[1].Name)
	require.Equal(t, int64(10), config.TracePipelineReferences[1].Generation)
}

func TestReadOTLPGatewayConfig_GetError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorGetClient{Client: fakeClient}

	_, err := ReadOTLPGatewayConfig(context.Background(), errorClient, "kyma-system")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get otlp gateway coordination configmap")
}

func TestReadOTLPGatewayConfig_InvalidYAML(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "invalid: yaml: [}",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	_, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal configmap")
}

func TestMultipleSignalTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `tracePipelines:
- name: trace-pipeline
  generation: 1
logPipelines:
- name: log-pipeline
  generation: 2
metricPipelines:
- name: metric-pipeline
  generation: 3
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)

	// Add another trace pipeline - should not affect other signal types
	err = AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "trace-pipeline-2", Generation: 5})
	require.NoError(t, err)

	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 2)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)

	// Remove trace pipeline - should not affect other signal types
	err = RemovePipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeTrace, "trace-pipeline")
	require.NoError(t, err)

	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Equal(t, "trace-pipeline-2", config.TracePipelineReferences[0].Name)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)
}

func TestWritePipelineReference_CreateNewConfigMap(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Len(t, refs, 1)
			require.Equal(t, "my-pipeline", refs[0].Name)
			require.Equal(t, int64(1), refs[0].Generation)
		})
	}
}

func TestWritePipelineReference_AddToExistingConfigMap(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			yamlKey := yamlKeyForSignalType(signalType)
			yamlData := fmt.Sprintf(`%s:
- name: existing-pipeline
  generation: 3
`, yamlKey)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.OTLPGatewayCoordinationConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					ConfigMapDataKey: yamlData,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

			err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{Name: "new-pipeline", Generation: 1})
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Len(t, refs, 2)

			foundExisting := false
			foundNew := false

			for _, ref := range refs {
				if ref.Name == "existing-pipeline" {
					require.Equal(t, int64(3), ref.Generation)

					foundExisting = true
				}

				if ref.Name == "new-pipeline" {
					require.Equal(t, int64(1), ref.Generation)

					foundNew = true
				}
			}

			require.True(t, foundExisting, "existing pipeline should be preserved")
			require.True(t, foundNew, "new pipeline should be added")
		})
	}
}

func TestWritePipelineReference_UpdateExisting(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			yamlKey := yamlKeyForSignalType(signalType)
			yamlData := fmt.Sprintf(`%s:
- name: my-pipeline
  generation: 5
`, yamlKey)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.OTLPGatewayCoordinationConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					ConfigMapDataKey: yamlData,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

			err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{Name: "my-pipeline", Generation: 10})
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Len(t, refs, 1)
			require.Equal(t, "my-pipeline", refs[0].Name)
			require.Equal(t, int64(10), refs[0].Generation)
		})
	}
}

func TestWritePipelineReference_GetError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorGetClient{Client: fakeClient}

	err := AddPipelineReference(context.Background(), errorClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get configmap")
}

func TestWritePipelineReference_InvalidYAMLInExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "invalid: yaml: [}",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal configmap")
}

func TestWritePipelineReference_CreateError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorCreateClient{Client: fakeClient}

	err := AddPipelineReference(context.Background(), errorClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create configmap")
}

func TestWritePipelineReference_UpdateError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "tracePipelines: []",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	errorClient := &errorUpdateClient{Client: fakeClient}

	err := AddPipelineReference(context.Background(), errorClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to update configmap")
}

func TestRemovePipelineReference_RemoveFromExisting(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			yamlKey := yamlKeyForSignalType(signalType)
			yamlData := fmt.Sprintf(`%s:
- name: pipeline-1
  generation: 5
- name: pipeline-2
  generation: 10
`, yamlKey)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.OTLPGatewayCoordinationConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					ConfigMapDataKey: yamlData,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

			err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", signalType, "pipeline-1")
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Len(t, refs, 1)
			require.Equal(t, "pipeline-2", refs[0].Name)
			require.Equal(t, int64(10), refs[0].Generation)
		})
	}
}

func TestRemovePipelineReference_Idempotent(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			yamlKey := yamlKeyForSignalType(signalType)
			yamlData := fmt.Sprintf(`%s:
- name: pipeline-1
  generation: 5
`, yamlKey)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.OTLPGatewayCoordinationConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					ConfigMapDataKey: yamlData,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

			// Remove non-existent pipeline (should not error)
			err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", signalType, "non-existent")
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Len(t, refs, 1)
			require.Equal(t, "pipeline-1", refs[0].Name)
		})
	}
}

func TestRemovePipelineReference_RemoveAll(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			yamlKey := yamlKeyForSignalType(signalType)
			yamlData := fmt.Sprintf(`%s:
- name: pipeline-1
  generation: 5
`, yamlKey)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.OTLPGatewayCoordinationConfigMap,
					Namespace: "kyma-system",
				},
				Data: map[string]string{
					ConfigMapDataKey: yamlData,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

			err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", signalType, "pipeline-1")
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Empty(t, refs)
		})
	}
}

func TestRemovePipelineReference_NoConfigMap(t *testing.T) {
	for _, signalType := range allSignalTypes {
		t.Run(string(signalType), func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", signalType, "pipeline-1")
			require.NoError(t, err)

			config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
			require.NoError(t, err)

			refs := getPipelineRefs(config, signalType)
			require.Empty(t, refs)
		})
	}
}

func TestMixedPipelineUpdates(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Add one pipeline per signal type
	err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "trace-1", Generation: 1})
	require.NoError(t, err)

	err = AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeLog, PipelineReferenceInput{Name: "log-1", Generation: 2})
	require.NoError(t, err)

	err = AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeMetric, PipelineReferenceInput{Name: "metric-1", Generation: 3})
	require.NoError(t, err)

	// Verify all exist
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)
	require.Equal(t, "trace-1", config.TracePipelineReferences[0].Name)
	require.Equal(t, "log-1", config.LogPipelineReferences[0].Name)
	require.Equal(t, "metric-1", config.MetricPipelineReferences[0].Name)

	// Update trace pipeline generation - others should be unchanged
	err = AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "trace-1", Generation: 5})
	require.NoError(t, err)

	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Equal(t, int64(5), config.TracePipelineReferences[0].Generation)
	require.Equal(t, int64(2), config.LogPipelineReferences[0].Generation)
	require.Equal(t, int64(3), config.MetricPipelineReferences[0].Generation)

	// Remove trace pipeline - others should be unchanged
	err = RemovePipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeTrace, "trace-1")
	require.NoError(t, err)

	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipelineReferences)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)
	require.Equal(t, "log-1", config.LogPipelineReferences[0].Name)
	require.Equal(t, "metric-1", config.MetricPipelineReferences[0].Name)

	// Remove metric pipeline - log should be unchanged
	err = RemovePipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalTypeMetric, "metric-1")
	require.NoError(t, err)

	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipelineReferences)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Empty(t, config.MetricPipelineReferences)
}

// Error client helpers for testing

type errorGetClient struct {
	client.Client
}

func (c *errorGetClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	return assert.AnError
}

type errorCreateClient struct {
	client.Client
}

func (c *errorCreateClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

func (c *errorCreateClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return assert.AnError
}

type errorUpdateClient struct {
	client.Client
}

func (c *errorUpdateClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *errorUpdateClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return assert.AnError
}

func TestAddPipelineReference_InvalidSignalType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalType("invalid"), PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid pipeline type")
}

func TestRemovePipelineReference_InvalidSignalType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", pipelines.SignalType("invalid"), "my-pipeline")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid pipeline type")
}

func TestCollectSecretVersions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("CollectsVersionFromExistingSecret", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "my-secret",
				Namespace:       "kyma-system",
				ResourceVersion: "12345",
			},
			Data: map[string][]byte{
				"endpoint": []byte("http://backend:4318"),
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

		refs := []telemetryv1beta1.SecretKeyRef{
			{
				Name:      "my-secret",
				Namespace: "kyma-system",
				Key:       "endpoint",
			},
		}

		versions, err := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.NoError(t, err)
		require.Len(t, versions, 1)
		require.Equal(t, "12345", versions["kyma-system/my-secret"])
	})

	t.Run("SkipsNotFoundSecret", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		refs := []telemetryv1beta1.SecretKeyRef{
			{
				Name:      "missing-secret",
				Namespace: "kyma-system",
				Key:       "endpoint",
			},
		}

		versions, err := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.NoError(t, err)
		require.Empty(t, versions)
	})

	t.Run("ReturnsErrorOnNonNotFoundGetFailure", func(t *testing.T) {
		refs := []telemetryv1beta1.SecretKeyRef{
			{Name: "my-secret", Namespace: "kyma-system", Key: "endpoint"},
		}

		errClient := &errorGetClient{}

		_, err := CollectSecretVersions(context.Background(), errClient, refs)

		require.Error(t, err)
	})

	t.Run("DeduplicatesByNamespace", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "my-secret",
				Namespace:       "kyma-system",
				ResourceVersion: "12345",
			},
			Data: map[string][]byte{
				"endpoint": []byte("http://backend:4318"),
				"token":    []byte("secret-token"),
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

		refs := []telemetryv1beta1.SecretKeyRef{
			{
				Name:      "my-secret",
				Namespace: "kyma-system",
				Key:       "endpoint",
			},
			{
				Name:      "my-secret",
				Namespace: "kyma-system",
				Key:       "token",
			},
		}

		versions, err := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.NoError(t, err)
		require.Len(t, versions, 1)
		require.Equal(t, "12345", versions["kyma-system/my-secret"])
	})

	t.Run("HandlesMultipleSecrets", func(t *testing.T) {
		secret1 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "secret1",
				Namespace:       "kyma-system",
				ResourceVersion: "111",
			},
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "secret2",
				Namespace:       "default",
				ResourceVersion: "222",
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret1, secret2).Build()

		refs := []telemetryv1beta1.SecretKeyRef{
			{Name: "secret1", Namespace: "kyma-system", Key: "key1"},
			{Name: "secret2", Namespace: "default", Key: "key2"},
		}

		versions, err := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.NoError(t, err)
		require.Len(t, versions, 2)
		require.Equal(t, "111", versions["kyma-system/secret1"])
		require.Equal(t, "222", versions["default/secret2"])
	})
}

func TestWritePipelineReferenceWithSecretVersions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("StoresSecretVersions", func(t *testing.T) {
		for _, signalType := range allSignalTypes {
			t.Run(string(signalType), func(t *testing.T) {
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

				secretVersions := map[string]string{
					"kyma-system/secret1": "111",
					"kyma-system/secret2": "222",
				}

				err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{
					Name:           "my-pipeline",
					Generation:     5,
					SecretVersions: secretVersions,
				})
				require.NoError(t, err)

				config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
				require.NoError(t, err)

				refs := getPipelineRefs(config, signalType)
				require.Len(t, refs, 1)
				require.Equal(t, "my-pipeline", refs[0].Name)
				require.Equal(t, int64(5), refs[0].Generation)
				require.Equal(t, secretVersions, refs[0].SecretVersions)
			})
		}
	})

	t.Run("UpdatesSecretVersionsOnRewrite", func(t *testing.T) {
		for _, signalType := range allSignalTypes {
			t.Run(string(signalType), func(t *testing.T) {
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

				err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{
					Name:       "my-pipeline",
					Generation: 5,
					SecretVersions: map[string]string{
						"kyma-system/secret1": "111",
					},
				})
				require.NoError(t, err)

				err = AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{
					Name:       "my-pipeline",
					Generation: 6,
					SecretVersions: map[string]string{
						"kyma-system/secret1": "222",
					},
				})
				require.NoError(t, err)

				config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
				require.NoError(t, err)

				refs := getPipelineRefs(config, signalType)
				require.Len(t, refs, 1)
				require.Equal(t, int64(6), refs[0].Generation)
				require.Equal(t, "222", refs[0].SecretVersions["kyma-system/secret1"])
			})
		}
	})

	t.Run("StoresEmptySecretVersionsWhenNone", func(t *testing.T) {
		for _, signalType := range allSignalTypes {
			t.Run(string(signalType), func(t *testing.T) {
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

				err := AddPipelineReference(context.Background(), fakeClient, "kyma-system", signalType, PipelineReferenceInput{
					Name:           "my-pipeline",
					Generation:     1,
					SecretVersions: nil,
				})
				require.NoError(t, err)

				config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
				require.NoError(t, err)

				refs := getPipelineRefs(config, signalType)
				require.Len(t, refs, 1)
				require.Nil(t, refs[0].SecretVersions)
			})
		}
	})
}

func TestWritePipelineReference_ConflictReturnsError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "tracePipelines: []",
		},
	}

	innerFake := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	c := &conflictOnUpdateClient{Client: innerFake}

	err := AddPipelineReference(context.Background(), c, "kyma-system", pipelines.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.True(t, apierrors.IsConflict(err))
}

// conflictOnUpdateClient always returns Conflict on Update.
type conflictOnUpdateClient struct {
	client.Client
}

func (c *conflictOnUpdateClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, obj.GetName(), fmt.Errorf("resource version mismatch"))
}
