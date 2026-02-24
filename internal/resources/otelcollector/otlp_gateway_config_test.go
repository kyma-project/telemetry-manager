package otelcollector

import (
	"context"
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
)

func TestReadOTLPGatewayConfig_ConfigMapNotExist(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.TracePipeline)
	require.Empty(t, config.LogPipeline)
	require.Empty(t, config.MetricPipeline)
}

func TestReadOTLPGatewayConfig_EmptyConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.TracePipeline)
}

func TestReadOTLPGatewayConfig_WithPipelines(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `TracePipeline:
- name: pipeline-1
  generation: 5
- name: pipeline-2
  generation: 10
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
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
	require.Len(t, config.TracePipeline, 2)
	require.Equal(t, "pipeline-1", config.TracePipeline[0].Name)
	require.Equal(t, int64(5), config.TracePipeline[0].Generation)
	require.Equal(t, "pipeline-2", config.TracePipeline[1].Name)
	require.Equal(t, int64(10), config.TracePipeline[1].Generation)
}

func TestWriteTracePipelineReference_CreateNewConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := WriteTracePipelineReference(context.Background(), fakeClient, "kyma-system", "my-pipeline", 1)
	require.NoError(t, err)

	// Verify ConfigMap was created
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 1)
	require.Equal(t, "my-pipeline", config.TracePipeline[0].Name)
	require.Equal(t, int64(1), config.TracePipeline[0].Generation)
}

func TestWriteTracePipelineReference_AddToExistingConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `TracePipeline:
- name: existing-pipeline
  generation: 3
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := WriteTracePipelineReference(context.Background(), fakeClient, "kyma-system", "new-pipeline", 1)
	require.NoError(t, err)

	// Verify both pipelines exist
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 2)

	// Check existing pipeline is preserved
	found := false

	for _, ref := range config.TracePipeline {
		if ref.Name == "existing-pipeline" {
			require.Equal(t, int64(3), ref.Generation)

			found = true

			break
		}
	}

	require.True(t, found, "existing pipeline should be preserved")

	// Check new pipeline is added
	found = false

	for _, ref := range config.TracePipeline {
		if ref.Name == "new-pipeline" {
			require.Equal(t, int64(1), ref.Generation)

			found = true

			break
		}
	}

	require.True(t, found, "new pipeline should be added")
}

func TestWriteTracePipelineReference_UpdateExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `TracePipeline:
- name: my-pipeline
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Update generation
	err := WriteTracePipelineReference(context.Background(), fakeClient, "kyma-system", "my-pipeline", 10)
	require.NoError(t, err)

	// Verify generation was updated
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 1)
	require.Equal(t, "my-pipeline", config.TracePipeline[0].Name)
	require.Equal(t, int64(10), config.TracePipeline[0].Generation)
}

func TestRemoveTracePipelineReference_RemoveFromExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `TracePipeline:
- name: pipeline-1
  generation: 5
- name: pipeline-2
  generation: 10
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := RemoveTracePipelineReference(context.Background(), fakeClient, "kyma-system", "pipeline-1")
	require.NoError(t, err)

	// Verify only pipeline-2 remains
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 1)
	require.Equal(t, "pipeline-2", config.TracePipeline[0].Name)
	require.Equal(t, int64(10), config.TracePipeline[0].Generation)
}

func TestRemoveTracePipelineReference_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `TracePipeline:
- name: pipeline-1
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Remove non-existent pipeline (should not error)
	err := RemoveTracePipelineReference(context.Background(), fakeClient, "kyma-system", "non-existent")
	require.NoError(t, err)

	// Verify pipeline-1 is still there
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 1)
	require.Equal(t, "pipeline-1", config.TracePipeline[0].Name)
}

func TestRemoveTracePipelineReference_RemoveAll(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `TracePipeline:
- name: pipeline-1
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := RemoveTracePipelineReference(context.Background(), fakeClient, "kyma-system", "pipeline-1")
	require.NoError(t, err)

	// Verify empty list
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipeline)
}

func TestRemoveTracePipelineReference_NoConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Remove from non-existent ConfigMap should create empty ConfigMap
	err := RemoveTracePipelineReference(context.Background(), fakeClient, "kyma-system", "pipeline-1")
	require.NoError(t, err)

	// Verify ConfigMap was created with empty list
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipeline)
}

func TestMultipleSignalTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create ConfigMap with multiple signal types
	yamlData := `TracePipeline:
- name: trace-pipeline
  generation: 1
LogPipeline:
- name: log-pipeline
  generation: 2
MetricPipeline:
- name: metric-pipeline
  generation: 3
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Read and verify all signal types
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 1)
	require.Len(t, config.LogPipeline, 1)
	require.Len(t, config.MetricPipeline, 1)

	// Add another trace pipeline - should not affect other signal types
	err = WriteTracePipelineReference(context.Background(), fakeClient, "kyma-system", "trace-pipeline-2", 5)
	require.NoError(t, err)

	// Verify trace pipeline added, others preserved
	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 2)
	require.Len(t, config.LogPipeline, 1)
	require.Len(t, config.MetricPipeline, 1)

	// Remove trace pipeline - should not affect other signal types
	err = RemoveTracePipelineReference(context.Background(), fakeClient, "kyma-system", "trace-pipeline")
	require.NoError(t, err)

	// Verify trace pipeline removed, others preserved
	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipeline, 1)
	require.Equal(t, "trace-pipeline-2", config.TracePipeline[0].Name)
	require.Len(t, config.LogPipeline, 1)
	require.Len(t, config.MetricPipeline, 1)
}

func TestReadOTLPGatewayConfig_GetError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorGetClient{Client: fakeClient}

	_, err := ReadOTLPGatewayConfig(context.Background(), errorClient, "kyma-system")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get otlp gateway configmap")
}

func TestReadOTLPGatewayConfig_InvalidYAML(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
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

func TestWriteTracePipelineReference_GetError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorGetClient{Client: fakeClient}

	err := WriteTracePipelineReference(context.Background(), errorClient, "kyma-system", "my-pipeline", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get configmap")
}

func TestWriteTracePipelineReference_InvalidYAMLInExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "invalid: yaml: [}",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := WriteTracePipelineReference(context.Background(), fakeClient, "kyma-system", "my-pipeline", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal configmap")
}

func TestWriteTracePipelineReference_CreateError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorCreateClient{Client: fakeClient}

	err := WriteTracePipelineReference(context.Background(), errorClient, "kyma-system", "my-pipeline", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create configmap")
}

func TestWriteTracePipelineReference_UpdateError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "TracePipeline: []",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	errorClient := &errorUpdateClient{Client: fakeClient}

	err := WriteTracePipelineReference(context.Background(), errorClient, "kyma-system", "my-pipeline", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to update configmap")
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
