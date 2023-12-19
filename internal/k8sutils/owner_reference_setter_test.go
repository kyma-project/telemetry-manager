package k8sutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewOwnerReferenceSetter(t *testing.T) {
	ctx := context.Background()
	interceptedClient := fake.NewClientBuilder().Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "owner"}}
	ownerRefSetter := NewOwnerReferenceSetter(interceptedClient, owner)

	t.Run("Create", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "dummy-1"}}
		require.NoError(t, ownerRefSetter.Create(ctx, obj))

		var got corev1.ConfigMap
		require.NoError(t, interceptedClient.Get(ctx, types.NamespacedName{Name: "dummy-1"}, &got))
		require.NotNil(t, got.OwnerReferences)
		require.Len(t, got.OwnerReferences, 1)
		require.Equal(t, owner.Name, got.OwnerReferences[0].Name)
	})

	t.Run("Update", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "dummy-2"}}
		require.NoError(t, interceptedClient.Create(ctx, obj))

		require.NoError(t, ownerRefSetter.Update(ctx, obj))

		var got corev1.ConfigMap
		require.NoError(t, interceptedClient.Get(ctx, types.NamespacedName{Name: "dummy-2"}, &got))
		require.NotNil(t, got.OwnerReferences)
		require.Len(t, got.OwnerReferences, 1)
		require.Equal(t, owner.Name, got.OwnerReferences[0].Name)
	})
}
