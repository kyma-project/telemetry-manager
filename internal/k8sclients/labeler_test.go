package k8sclients

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

var testLabels = commonresources.MakeDefaultLabels("test-collector", "gateway")

func TestLabeler_Create(t *testing.T) {
	inner := fake.NewClientBuilder().Build()
	labeler := NewLabeler(inner, testLabels)

	t.Run("adds default labels to object without labels", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "no-labels", Namespace: "default"}}
		require.NoError(t, labeler.Create(t.Context(), obj))

		var got corev1.ConfigMap
		require.NoError(t, inner.Get(t.Context(), types.NamespacedName{Name: "no-labels", Namespace: "default"}, &got))
		requireDefaultLabels(t, got.Labels)
	})

	t.Run("preserves existing labels", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      "with-labels",
			Namespace: "default",
			Labels:    map[string]string{"custom": "value"},
		}}
		require.NoError(t, labeler.Create(t.Context(), obj))

		var got corev1.ConfigMap
		require.NoError(t, inner.Get(t.Context(), types.NamespacedName{Name: "with-labels", Namespace: "default"}, &got))
		requireDefaultLabels(t, got.Labels)
		require.Equal(t, "value", got.Labels["custom"])
	})
}

func TestLabeler_Update(t *testing.T) {
	inner := fake.NewClientBuilder().Build()
	labeler := NewLabeler(inner, testLabels)

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "update-test", Namespace: "default"}}
	require.NoError(t, inner.Create(t.Context(), obj))

	require.NoError(t, labeler.Update(t.Context(), obj))

	var got corev1.ConfigMap
	require.NoError(t, inner.Get(t.Context(), types.NamespacedName{Name: "update-test", Namespace: "default"}, &got))
	requireDefaultLabels(t, got.Labels)
}

func TestLabeler_Patch(t *testing.T) {
	inner := fake.NewClientBuilder().Build()
	labeler := NewLabeler(inner, testLabels)

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "patch-test", Namespace: "default"}}
	require.NoError(t, inner.Create(t.Context(), obj))

	obj.Labels = map[string]string{"extra": "label"}
	require.NoError(t, labeler.Patch(t.Context(), obj, client.MergeFrom(obj.DeepCopy())))

	var got corev1.ConfigMap
	require.NoError(t, inner.Get(t.Context(), types.NamespacedName{Name: "patch-test", Namespace: "default"}, &got))
	requireDefaultLabels(t, got.Labels)
}

func TestLabeler_Get(t *testing.T) {
	inner := fake.NewClientBuilder().Build()
	labeler := NewLabeler(inner, testLabels)

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "get-test", Namespace: "default"}}
	require.NoError(t, inner.Create(t.Context(), obj))

	var got corev1.ConfigMap
	require.NoError(t, labeler.Get(t.Context(), types.NamespacedName{Name: "get-test", Namespace: "default"}, &got))
	require.Equal(t, "get-test", got.Name)
}

func requireDefaultLabels(t *testing.T, labels map[string]string) {
	t.Helper()

	require.Equal(t, commonresources.LabelValueKymaModule, labels[commonresources.LabelKeyKymaModule])
	require.Equal(t, commonresources.LabelValueK8sPartOf, labels[commonresources.LabelKeyK8sPartOf])
	require.Equal(t, commonresources.LabelValueK8sManagedBy, labels[commonresources.LabelKeyK8sManagedBy])
	require.Equal(t, "test-collector", labels[commonresources.LabelKeyK8sName])
	require.Equal(t, "gateway", labels[commonresources.LabelKeyK8sComponent])
}
