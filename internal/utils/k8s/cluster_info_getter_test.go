package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterInfoGetter(t *testing.T) {
	t.Run("Gardener cluster", func(t *testing.T) {
		shootInfo := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shoot-info", Namespace: "kube-system"},
			Data: map[string]string{
				"shootName": "test-cluster",
				"provider":  "test-provider",
			},
		}

		fakeClient := fake.NewClientBuilder().WithObjects(shootInfo).Build()

		clusterInfo := GetGardenerShootInfo(context.Background(), fakeClient)

		require.Equal(t, clusterInfo.ClusterName, "test-cluster")
		require.Equal(t, clusterInfo.CloudProvider, "test-provider")
	})

	t.Run("Gardener converged cloud", func(t *testing.T) {
		shootInfo := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shoot-info", Namespace: "kube-system"},
			Data: map[string]string{
				"shootName": "test-cluster",
				"provider":  "openstack",
			},
		}

		fakeClient := fake.NewClientBuilder().WithObjects(shootInfo).Build()

		clusterInfo := GetGardenerShootInfo(context.Background(), fakeClient)

		require.Equal(t, clusterInfo.ClusterName, "test-cluster")
		require.Equal(t, clusterInfo.CloudProvider, "sap")
	})

	t.Run("Non Gardener cluster", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithObjects().Build()

		clusterInfo := GetGardenerShootInfo(context.Background(), fakeClient)

		require.Equal(t, clusterInfo.ClusterName, "${KUBERNETES_SERVICE_HOST}")
		require.Equal(t, clusterInfo.CloudProvider, "")
	})
}
