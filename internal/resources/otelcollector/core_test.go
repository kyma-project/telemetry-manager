package otelcollector

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// makeFailingClient creates a fake client that fails for specific object types
func makeFailingClient(scheme *runtime.Scheme, failType string) client.Client {
	shouldFail := func(obj client.Object) bool {
		switch failType {
		case "serviceaccount":
			_, ok := obj.(*corev1.ServiceAccount)
			return ok
		case "clusterrole":
			_, ok := obj.(*rbacv1.ClusterRole)
			return ok
		case "clusterrolebinding":
			_, ok := obj.(*rbacv1.ClusterRoleBinding)
			return ok
		case "role":
			_, ok := obj.(*rbacv1.Role)
			return ok
		case "rolebinding":
			_, ok := obj.(*rbacv1.RoleBinding)
			return ok
		case "service":
			svc, ok := obj.(*corev1.Service)
			return ok && svc.Name != ""
		}

		return false
	}

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if shouldFail(obj) {
					return errors.New(failType + " error")
				}

				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if shouldFail(obj) {
					return errors.New(failType + " error")
				}

				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()
}

func TestApplyCommonResources_ServiceAccountError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c := makeFailingClient(scheme, "serviceaccount")
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	err := applyCommonResources(context.Background(), c, name, rbac{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create service account")
}

func TestApplyCommonResources_ClusterRoleError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c := makeFailingClient(scheme, "clusterrole")
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
	testRBAC := rbac{
		clusterRole: &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cr"},
		},
	}

	err := applyCommonResources(context.Background(), c, name, testRBAC)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create cluster role")
}

func TestApplyCommonResources_ClusterRoleBindingError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c := makeFailingClient(scheme, "clusterrolebinding")
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
	testRBAC := rbac{
		clusterRoleBinding: &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-crb"},
		},
	}

	err := applyCommonResources(context.Background(), c, name, testRBAC)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create cluster role binding")
}

func TestApplyCommonResources_RoleError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c := makeFailingClient(scheme, "role")
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
	testRBAC := rbac{
		role: &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "test-role", Namespace: "test-ns"},
		},
	}

	err := applyCommonResources(context.Background(), c, name, testRBAC)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create role")
}

func TestApplyCommonResources_RoleBindingError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c := makeFailingClient(scheme, "rolebinding")
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
	testRBAC := rbac{
		roleBinding: &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rb", Namespace: "test-ns"},
		},
	}

	err := applyCommonResources(context.Background(), c, name, testRBAC)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create role binding")
}

func TestApplyCommonResources_MetricsServiceError(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c := makeFailingClient(scheme, "service")
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	err := applyCommonResources(context.Background(), c, name, rbac{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create metrics service")
}

func TestDeleteCommonResources_ErrorHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("collects all deletion errors", func(t *testing.T) {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					// Return error for all deletions except NotFound
					return errors.New("deletion error")
				},
			}).
			Build()

		err := deleteCommonResources(context.Background(), c, name)
		require.Error(t, err)
		// Should contain multiple errors
		require.Contains(t, err.Error(), "deletion error")
	})
}

func TestMakeVPA(t *testing.T) {
	tests := []struct {
		name              string
		minAllowedMemory  resource.Quantity
		maxAllowedMemory  resource.Quantity
		expectedMaxMemory resource.Quantity
	}{
		{
			name:              "max greater than min",
			minAllowedMemory:  resource.MustParse("128Mi"),
			maxAllowedMemory:  resource.MustParse("512Mi"),
			expectedMaxMemory: resource.MustParse("512Mi"),
		},
		{
			name:              "max less than min - clamps to min",
			minAllowedMemory:  resource.MustParse("512Mi"),
			maxAllowedMemory:  resource.MustParse("128Mi"),
			expectedMaxMemory: resource.MustParse("512Mi"),
		},
		{
			name:              "max equals min",
			minAllowedMemory:  resource.MustParse("256Mi"),
			maxAllowedMemory:  resource.MustParse("256Mi"),
			expectedMaxMemory: resource.MustParse("256Mi"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := types.NamespacedName{Name: "test-vpa", Namespace: "test-ns"}
			vpa := makeVPA(name, tt.minAllowedMemory, tt.maxAllowedMemory)

			require.Equal(t, "test-vpa", vpa.Name)
			require.Equal(t, "test-ns", vpa.Namespace)
			require.Equal(t, "DaemonSet", vpa.Spec.TargetRef.Kind)

			policy := vpa.Spec.ResourcePolicy.ContainerPolicies[0]
			actualMaxMemory := policy.MaxAllowed[corev1.ResourceMemory]
			require.Equal(t, tt.expectedMaxMemory.Value(), actualMaxMemory.Value())
		})
	}
}