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

func TestApplyYAML_UpdatesExistingResources(t *testing.T) {
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

	// Apply the YAML (should update the existing ConfigMap via server-side apply)
	ctx := context.Background()
	err := applyYAML(ctx, client, yamlContent)
	require.NoError(t, err, "applyYAML should succeed and update existing resource")

	// Verify the ConfigMap was updated
	cm := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "test-cm",
		Namespace: "default",
	}, cm)
	require.NoError(t, err)

	// Server-side apply replaces the data with the new value
	require.Contains(t, cm.Data, "new-key", "should have new key after update")
	require.Equal(t, "new-value", cm.Data["new-key"])
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
	err := applyYAML(ctx, client, yamlContent)
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
	err := applyYAML(ctx, client, yamlContent)
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
	err := applyYAML(ctx, client, yamlContent)
	require.NoError(t, err, "applyYAML should skip empty objects")

	// Verify the ConfigMap was created
	cm := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      "cm",
		Namespace: "default",
	}, cm)
	require.NoError(t, err)
}
