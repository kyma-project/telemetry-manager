package kubeprep

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// mockTestingT is a mock implementation of TestingT for tests that need to pass
// a context that doesn't come from t.Context() (e.g., for testing with specific contexts)
type mockTestingT struct {
	*testing.T
	ctx context.Context
}

func (m *mockTestingT) Context() context.Context {
	return m.ctx
}

func TestApplyYAML_SkipsExistingResources(t *testing.T) {
	// Create a fake client with an existing ConfigMap
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cm",
			Namespace:       "default",
			ResourceVersion: "12345",
		},
		Data: map[string]string{
			"old-key": "old-value",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingCM).
		Build()

	// YAML with the same ConfigMap but different data
	yamlContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  new-key: new-value
`

	// Apply the YAML (should skip the existing ConfigMap)
	ctx := context.Background()
	err := applyYAML(ctx, client, t, yamlContent)
	require.NoError(t, err, "applyYAML should succeed and skip existing resource")

	// Verify the ConfigMap was NOT updated (original data preserved)
	cm := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "test-cm",
		Namespace: "default",
	}, cm)
	require.NoError(t, err)

	// Verify old data is still there (resource was skipped, not updated)
	require.Contains(t, cm.Data, "old-key")
	require.Equal(t, "old-value", cm.Data["old-key"])
	require.NotContains(t, cm.Data, "new-key", "should not have new key - resource was skipped")
}

func TestApplyYAML_CreateNewResource(t *testing.T) {
	// Create a fake client with no existing resources
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// YAML with a new ConfigMap
	yamlContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: new-cm
  namespace: default
data:
  key: value
`

	// Apply the YAML (should create a new ConfigMap)
	ctx := context.Background()
	err := applyYAML(ctx, client, t, yamlContent)
	require.NoError(t, err, "applyYAML should succeed when creating new resource")

	// Verify the ConfigMap was created
	cm := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "new-cm",
		Namespace: "default",
	}, cm)
	require.NoError(t, err)

	// Verify data
	require.Contains(t, cm.Data, "key")
	require.Equal(t, "value", cm.Data["key"])
}

func TestApplyYAML_MultipleResources(t *testing.T) {
	// Create a fake client with no existing resources
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// YAML with multiple resources
	yamlContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  key1: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: default
data:
  key2: value2
`

	// Apply the YAML
	ctx := context.Background()
	err := applyYAML(ctx, client, t, yamlContent)
	require.NoError(t, err, "applyYAML should succeed with multiple resources")

	// Verify both ConfigMaps were created
	cm1 := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "cm1",
		Namespace: "default",
	}, cm1)
	require.NoError(t, err)
	require.Equal(t, "value1", cm1.Data["key1"])

	cm2 := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "cm2",
		Namespace: "default",
	}, cm2)
	require.NoError(t, err)
	require.Equal(t, "value2", cm2.Data["key2"])
}

func TestApplyYAML_SkipsEmptyObjects(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// YAML with empty documents
	yamlContent := `
---
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: default
data:
  key: value
---
---
`

	// Apply the YAML (should skip empty objects)
	ctx := context.Background()
	err := applyYAML(ctx, client, t, yamlContent)
	require.NoError(t, err, "applyYAML should skip empty objects")

	// Verify the ConfigMap was created
	cm := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "cm",
		Namespace: "default",
	}, cm)
	require.NoError(t, err)
}
