package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterInfoGetter(t *testing.T) {
	t.Run("Gardener cluster", func(t *testing.T) {
		shootInfo := &corev1.ConfigMap{
			Name: "shoot-info", Namespace: "kube-system",
			Data: map[string]string{
				"provider": "test-provider",
			},
		}

		fakeClient := fake.NewClientBuilder().WithObjects(shootInfo).Build()

		clusterInfo := GetGardenerShootInfo(t.Context(), fakeClient)

		require.Equal(t, clusterInfo.ClusterName, "${KUBERNETES_SERVICE_HOST}")
		require.Equal(t, clusterInfo.CloudProvider, "test-provider")
	})

	t.Run("Gardener converged cloud", func(t *testing.T) {
		shootInfo := &corev1.ConfigMap{
			Name: "shoot-info", Namespace: "kube-system",
			Data: map[string]string{
				"provider": "openstack",
			},
		}

		fakeClient := fake.NewClientBuilder().WithObjects(shootInfo).Build()

		clusterInfo := GetGardenerShootInfo(t.Context(), fakeClient)

		require.Equal(t, clusterInfo.ClusterName, "${KUBERNETES_SERVICE_HOST}")
		require.Equal(t, clusterInfo.CloudProvider, "sap")
	})

	t.Run("Non Gardener cluster", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithObjects().Build()

		clusterInfo := GetGardenerShootInfo(t.Context(), fakeClient)

		require.Equal(t, clusterInfo.ClusterName, "${KUBERNETES_SERVICE_HOST}")
		require.Equal(t, clusterInfo.CloudProvider, "")
	})
}

func TestGetClusterUID(t *testing.T) {
	t.Run("returns kube-system namespace UID", func(t *testing.T) {
		kubeSystemNs := &corev1.Namespace{
			Name: "kube-system",
			UID:  "test-cluster-uid-12345",
		}

		fakeClient := fake.NewClientBuilder().WithObjects(kubeSystemNs).Build()

		uid, err := GetClusterUID(t.Context(), fakeClient)

		require.NoError(t, err)
		require.Equal(t, "test-cluster-uid-12345", uid)
	})

	t.Run("returns error when kube-system namespace not found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().Build()

		uid, err := GetClusterUID(t.Context(), fakeClient)

		require.Error(t, err)
		require.Empty(t, uid)
	})
}
