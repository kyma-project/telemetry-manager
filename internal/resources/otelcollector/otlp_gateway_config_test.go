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

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

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
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.TracePipelineReferences)
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
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
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

func TestWriteTracePipelineReference_CreateNewConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.NoError(t, err)

	// Verify ConfigMap was created
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Equal(t, "my-pipeline", config.TracePipelineReferences[0].Name)
	require.Equal(t, int64(1), config.TracePipelineReferences[0].Generation)
}

func TestWriteTracePipelineReference_AddToExistingConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `tracePipelines:
- name: existing-pipeline
  generation: 3
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "new-pipeline", Generation: 1})
	require.NoError(t, err)

	// Verify both pipelines exist
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 2)

	// Check existing pipeline is preserved
	found := false

	for _, ref := range config.TracePipelineReferences {
		if ref.Name == "existing-pipeline" {
			require.Equal(t, int64(3), ref.Generation)

			found = true

			break
		}
	}

	require.True(t, found, "existing pipeline should be preserved")

	// Check new pipeline is added
	found = false

	for _, ref := range config.TracePipelineReferences {
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

	yamlData := `tracePipelines:
- name: my-pipeline
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Update generation
	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 10})
	require.NoError(t, err)

	// Verify generation was updated
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Equal(t, "my-pipeline", config.TracePipelineReferences[0].Name)
	require.Equal(t, int64(10), config.TracePipelineReferences[0].Generation)
}

func TestRemoveTracePipelineReference_RemoveFromExisting(t *testing.T) {
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
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, "pipeline-1")
	require.NoError(t, err)

	// Verify only pipeline-2 remains
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Equal(t, "pipeline-2", config.TracePipelineReferences[0].Name)
	require.Equal(t, int64(10), config.TracePipelineReferences[0].Generation)
}

func TestRemoveTracePipelineReference_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `tracePipelines:
- name: pipeline-1
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Remove non-existent pipeline (should not error)
	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, "non-existent")
	require.NoError(t, err)

	// Verify pipeline-1 is still there
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Equal(t, "pipeline-1", config.TracePipelineReferences[0].Name)
}

func TestRemoveTracePipelineReference_RemoveAll(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `tracePipelines:
- name: pipeline-1
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, "pipeline-1")
	require.NoError(t, err)

	// Verify empty list
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipelineReferences)
}

func TestRemoveTracePipelineReference_NoConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Remove from non-existent ConfigMap should create empty ConfigMap
	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, "pipeline-1")
	require.NoError(t, err)

	// Verify ConfigMap was created with empty list
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipelineReferences)
}

func TestMultipleSignalTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create ConfigMap with multiple signal types
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
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
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
	require.Len(t, config.TracePipelineReferences, 1)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)

	// Add another trace pipeline - should not affect other signal types
	err = WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "trace-pipeline-2", Generation: 5})
	require.NoError(t, err)

	// Verify trace pipeline added, others preserved
	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 2)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)

	// Remove trace pipeline - should not affect other signal types
	err = RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, "trace-pipeline")
	require.NoError(t, err)

	// Verify trace pipeline removed, others preserved
	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Equal(t, "trace-pipeline-2", config.TracePipelineReferences[0].Name)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Len(t, config.MetricPipelineReferences, 1)
}

func TestReadOTLPGatewayConfig_GetError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorGetClient{Client: fakeClient}

	_, err := ReadOTLPGatewayConfig(context.Background(), errorClient, "kyma-system")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get otlp gateway pipelines sync configmap")
}

func TestReadOTLPGatewayConfig_InvalidYAML(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
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

	err := WritePipelineReference(context.Background(), errorClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get configmap")
}

func TestWriteTracePipelineReference_InvalidYAMLInExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "invalid: yaml: [}",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal configmap")
}

func TestWriteTracePipelineReference_CreateError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	errorClient := &errorCreateClient{Client: fakeClient}

	err := WritePipelineReference(context.Background(), errorClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create configmap")
}

func TestWriteTracePipelineReference_UpdateError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: "tracePipelines: []",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	errorClient := &errorUpdateClient{Client: fakeClient}

	err := WritePipelineReference(context.Background(), errorClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "my-pipeline", Generation: 1})
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

func TestWriteLogPipelineReference_CreateNewConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, PipelineReferenceInput{Name: "my-log-pipeline", Generation: 1})
	require.NoError(t, err)

	// Verify ConfigMap was created
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Equal(t, "my-log-pipeline", config.LogPipelineReferences[0].Name)
	require.Equal(t, int64(1), config.LogPipelineReferences[0].Generation)
}

func TestWriteLogPipelineReference_AddToExistingConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `logPipelines:
- name: existing-log-pipeline
  generation: 3
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, PipelineReferenceInput{Name: "new-log-pipeline", Generation: 1})
	require.NoError(t, err)

	// Verify both pipelines exist
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.LogPipelineReferences, 2)

	// Check existing pipeline is preserved
	found := false

	for _, ref := range config.LogPipelineReferences {
		if ref.Name == "existing-log-pipeline" {
			require.Equal(t, int64(3), ref.Generation)

			found = true

			break
		}
	}

	require.True(t, found, "existing log pipeline should be preserved")

	// Check new pipeline is added
	found = false

	for _, ref := range config.LogPipelineReferences {
		if ref.Name == "new-log-pipeline" {
			require.Equal(t, int64(1), ref.Generation)

			found = true

			break
		}
	}

	require.True(t, found, "new log pipeline should be added")
}

func TestWriteLogPipelineReference_UpdateExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `logPipelines:
- name: my-log-pipeline
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Update generation
	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, PipelineReferenceInput{Name: "my-log-pipeline", Generation: 10})
	require.NoError(t, err)

	// Verify generation was updated
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Equal(t, "my-log-pipeline", config.LogPipelineReferences[0].Name)
	require.Equal(t, int64(10), config.LogPipelineReferences[0].Generation)
}

func TestRemoveLogPipelineReference_RemoveFromExisting(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `logPipelines:
- name: log-pipeline-1
  generation: 5
- name: log-pipeline-2
  generation: 10
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, "log-pipeline-1")
	require.NoError(t, err)

	// Verify only log-pipeline-2 remains
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Equal(t, "log-pipeline-2", config.LogPipelineReferences[0].Name)
	require.Equal(t, int64(10), config.LogPipelineReferences[0].Generation)
}

func TestRemoveLogPipelineReference_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	yamlData := `logPipelines:
- name: log-pipeline-1
  generation: 5
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayPipelinesSyncConfigMap,
			Namespace: "kyma-system",
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// Remove non-existent pipeline (should not error)
	err := RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, "non-existent")
	require.NoError(t, err)

	// Verify log-pipeline-1 is still there
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Equal(t, "log-pipeline-1", config.LogPipelineReferences[0].Name)
}

func TestMixedTraceAndLogPipelineUpdates(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Add trace pipeline
	err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "trace-1", Generation: 1})
	require.NoError(t, err)

	// Add log pipeline
	err = WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, PipelineReferenceInput{Name: "log-1", Generation: 2})
	require.NoError(t, err)

	// Verify both exist
	config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Len(t, config.TracePipelineReferences, 1)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Equal(t, "trace-1", config.TracePipelineReferences[0].Name)
	require.Equal(t, "log-1", config.LogPipelineReferences[0].Name)

	// Update trace pipeline
	err = WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{Name: "trace-1", Generation: 5})
	require.NoError(t, err)

	// Verify trace updated, log unchanged
	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Equal(t, int64(5), config.TracePipelineReferences[0].Generation)
	require.Equal(t, int64(2), config.LogPipelineReferences[0].Generation)

	// Remove trace pipeline
	err = RemovePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, "trace-1")
	require.NoError(t, err)

	// Verify trace removed, log unchanged
	config, err = ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
	require.NoError(t, err)
	require.Empty(t, config.TracePipelineReferences)
	require.Len(t, config.LogPipelineReferences, 1)
	require.Equal(t, "log-1", config.LogPipelineReferences[0].Name)
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

		versions := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.Len(t, versions, 1)
		require.Equal(t, "12345", versions["kyma-system/my-secret"])
	})

	t.Run("SkipsMissingSecret", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		refs := []telemetryv1beta1.SecretKeyRef{
			{
				Name:      "missing-secret",
				Namespace: "kyma-system",
				Key:       "endpoint",
			},
		}

		versions := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.Empty(t, versions)
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

		// Multiple refs to same secret
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

		versions := CollectSecretVersions(context.Background(), fakeClient, refs)

		// Should only have one entry per unique secret
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

		versions := CollectSecretVersions(context.Background(), fakeClient, refs)

		require.Len(t, versions, 2)
		require.Equal(t, "111", versions["kyma-system/secret1"])
		require.Equal(t, "222", versions["default/secret2"])
	})
}

func TestWritePipelineReferenceWithSecretVersions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("StoresSecretVersionsForTracePipeline", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		secretVersions := map[string]string{
			"kyma-system/secret1": "111",
			"kyma-system/secret2": "222",
		}

		err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{
			Name:           "my-pipeline",
			Generation:     5,
			SecretVersions: secretVersions,
		})
		require.NoError(t, err)

		config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
		require.NoError(t, err)
		require.Len(t, config.TracePipelineReferences, 1)
		require.Equal(t, "my-pipeline", config.TracePipelineReferences[0].Name)
		require.Equal(t, int64(5), config.TracePipelineReferences[0].Generation)
		require.Equal(t, secretVersions, config.TracePipelineReferences[0].SecretVersions)
	})

	t.Run("UpdatesSecretVersionsOnRewrite", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		// First write with version 111
		err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{
			Name:       "my-pipeline",
			Generation: 5,
			SecretVersions: map[string]string{
				"kyma-system/secret1": "111",
			},
		})
		require.NoError(t, err)

		// Update with version 222
		err = WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeTrace, PipelineReferenceInput{
			Name:       "my-pipeline",
			Generation: 6,
			SecretVersions: map[string]string{
				"kyma-system/secret1": "222",
			},
		})
		require.NoError(t, err)

		config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
		require.NoError(t, err)
		require.Len(t, config.TracePipelineReferences, 1)
		require.Equal(t, int64(6), config.TracePipelineReferences[0].Generation)
		require.Equal(t, "222", config.TracePipelineReferences[0].SecretVersions["kyma-system/secret1"])
	})

	t.Run("StoresEmptySecretVersionsWhenNone", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		err := WritePipelineReference(context.Background(), fakeClient, "kyma-system", common.SignalTypeLog, PipelineReferenceInput{
			Name:           "my-log-pipeline",
			Generation:     1,
			SecretVersions: nil,
		})
		require.NoError(t, err)

		config, err := ReadOTLPGatewayConfig(context.Background(), fakeClient, "kyma-system")
		require.NoError(t, err)
		require.Len(t, config.LogPipelineReferences, 1)
		require.Nil(t, config.LogPipelineReferences[0].SecretVersions)
	})
}
