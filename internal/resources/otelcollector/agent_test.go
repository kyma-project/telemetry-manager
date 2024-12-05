package otelcollector

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

var (
	agentNamespace = "my-namespace"
	agentName      = "my-agent"
	agentCfg       = "dummy otel collector config"
)

func TestApplyAgentResources(t *testing.T) {
	var objects []client.Object
	client := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			objects = append(objects, obj)
			return nil
		},
	}).Build()

	image := "otel/collector:latest"
	namespace := "kyma-system"
	priorityClassName := "normal"
	sut := NewMetricAgentApplierDeleter(image, namespace, priorityClassName)

	err := sut.ApplyResources(context.Background(), client, AgentApplyOptions{
		AllowedPorts:        []int32{},
		CollectorConfigYAML: "dummy",
	})

	bytes, err := testutils.MarshalYAML(objects)
	require.NoError(t, err)

	goldenFileBytes, err := os.ReadFile("testdata/metric-agent.yaml")
	require.NoError(t, err)

	require.Equal(t, goldenFileBytes, bytes)
}

func TestDeleteAgentResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	sut := AgentApplierDeleter{
		baseName:  agentName,
		namespace: agentNamespace,
		rbac:      createAgentRBAC(),
	}

	// Create agent resources before testing deletion
	err := sut.ApplyResources(ctx, client, AgentApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: agentCfg,
	})
	require.NoError(t, err)

	// Delete agent resources
	err = sut.DeleteResources(ctx, client)
	require.NoError(t, err)

	t.Run("should delete service account", func(t *testing.T) {
		var serviceAccount corev1.ServiceAccount
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &serviceAccount)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete cluster role", func(t *testing.T) {
		var clusterRole rbacv1.ClusterRole
		err := client.Get(ctx, types.NamespacedName{Name: agentName}, &clusterRole)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete cluster role binding", func(t *testing.T) {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &clusterRoleBinding)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete metrics service", func(t *testing.T) {
		var service corev1.Service
		err := client.Get(ctx, types.NamespacedName{Name: agentName + "-metrics", Namespace: agentNamespace}, &service)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete network policy", func(t *testing.T) {
		var networkPolicy networkingv1.NetworkPolicy
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &networkPolicy)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete collector config configmap", func(t *testing.T) {
		var configMap corev1.ConfigMap
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &configMap)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete daemonset", func(t *testing.T) {
		var daemonSet appsv1.DaemonSet
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &daemonSet)
		require.True(t, apierrors.IsNotFound(err))
	})
}

func createAgentRBAC() rbac {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentName,
			Namespace: agentNamespace,
			Labels:    commonresources.MakeDefaultLabels(agentName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"test"},
				Resources: []string{"test"},
				Verbs:     []string{"test"},
			},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentName,
			Namespace: agentNamespace,
			Labels:    commonresources.MakeDefaultLabels(agentName),
		},
		Subjects: []rbacv1.Subject{{Name: agentName, Namespace: agentNamespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     agentName,
		},
	}

	return rbac{
		clusterRole:        clusterRole,
		clusterRoleBinding: clusterRoleBinding,
		role:               nil,
		roleBinding:        nil,
	}
}
