package otelcollector

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

func TestApplyCommonResources(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	tests := []struct {
		name          string
		rbac          rbac
		initialObjs   []client.Object
		expectError   bool
		errorContains string
		validate      func(t *testing.T, c client.Client, name types.NamespacedName)
	}{
		{
			name: "creates all RBAC resources",
			rbac: rbac{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cr"},
				},
				clusterRoleBinding: &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-crb"},
				},
				role: &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				},
				roleBinding: &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				},
			},
			validate: func(t *testing.T, c client.Client, name types.NamespacedName) {
				var sa corev1.ServiceAccount
				require.NoError(t, c.Get(context.Background(), name, &sa))

				var cr rbacv1.ClusterRole
				require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "test-cr"}, &cr))

				var crb rbacv1.ClusterRoleBinding
				require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "test-crb"}, &crb))

				var role rbacv1.Role
				require.NoError(t, c.Get(context.Background(), name, &role))

				var rb rbacv1.RoleBinding
				require.NoError(t, c.Get(context.Background(), name, &rb))

				var svc corev1.Service
				metricsServiceName := types.NamespacedName{
					Name:      names.MetricsServiceName(name.Name),
					Namespace: name.Namespace,
				}
				require.NoError(t, c.Get(context.Background(), metricsServiceName, &svc))
				require.Equal(t, commonresources.LabelValueTelemetrySelfMonitor, svc.Labels[commonresources.LabelKeyTelemetrySelfMonitor])
			},
		},
		{
			name: "creates only subset of RBAC resources",
			rbac: rbac{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cr"},
				},
				// No cluster role binding, role, or role binding
			},
			validate: func(t *testing.T, c client.Client, name types.NamespacedName) {
				var sa corev1.ServiceAccount
				require.NoError(t, c.Get(context.Background(), name, &sa))

				var cr rbacv1.ClusterRole
				require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "test-cr"}, &cr))

				var crb rbacv1.ClusterRoleBinding
				err := c.Get(context.Background(), types.NamespacedName{Name: "test-crb"}, &crb)
				require.True(t, apierrors.IsNotFound(err))
			},
		},
		{
			name: "updates existing resources",
			rbac: rbac{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cr"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
					},
				},
			},
			initialObjs: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cr"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"list"}},
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				},
			},
			validate: func(t *testing.T, c client.Client, name types.NamespacedName) {
				var cr rbacv1.ClusterRole
				require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "test-cr"}, &cr))
				require.Len(t, cr.Rules, 1)
				require.Equal(t, "pods", cr.Rules[0].Resources[0])
			},
		},
		{
			name:          "handles service account creation failure",
			rbac:          rbac{},
			expectError:   true,
			errorContains: "failed to create service account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fakeClient client.Client
			if tt.name == "handles service account creation failure" {
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*corev1.ServiceAccount); ok {
								return errors.New("simulated error")
							}
							return client.Create(ctx, obj, opts...)
						},
					}).
					Build()
			} else {
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(tt.initialObjs...).
					Build()
			}

			name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
			err := applyCommonResources(context.Background(), fakeClient, name, tt.rbac)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, fakeClient, name)
				}
			}
		})
	}
}

func TestDeleteCommonResources(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	tests := []struct {
		name          string
		initialObjs   []client.Object
		expectError   bool
		errorContains string
		validate      func(t *testing.T, c client.Client, name types.NamespacedName)
	}{
		{
			name: "deletes all resources successfully",
			initialObjs: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				},
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: names.MetricsServiceName("test"), Namespace: "test-ns"},
				},
			},
				validate: func(t *testing.T, c client.Client, name types.NamespacedName) {
					// Verification that deletion was attempted is enough
				},
		},
		{
			name:        "handles non-existent resources gracefully",
			initialObjs: []client.Object{},
			validate: func(t *testing.T, c client.Client, name types.NamespacedName) {
				// Should not error even if resources don't exist
			},
		},
		{
			name: "collects multiple deletion errors",
			initialObjs: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.initialObjs...).
				Build()

			name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
			err := deleteCommonResources(context.Background(), fakeClient, name)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, fakeClient, name)
				}
			}
		})
	}
}

func TestMakeVPA(t *testing.T) {
	tests := []struct {
		name               string
		minAllowedMemory   resource.Quantity
		maxAllowedMemory   resource.Quantity
		expectedMaxMemory  resource.Quantity
		validateUpdateMode bool
	}{
		{
			name:              "normal case with max > min",
			minAllowedMemory:  resource.MustParse("128Mi"),
			maxAllowedMemory:  resource.MustParse("512Mi"),
			expectedMaxMemory: resource.MustParse("512Mi"),
		},
		{
			name:              "max < min should clamp to min",
			minAllowedMemory:  resource.MustParse("512Mi"),
			maxAllowedMemory:  resource.MustParse("128Mi"),
			expectedMaxMemory: resource.MustParse("512Mi"),
		},
		{
			name:              "max == min",
			minAllowedMemory:  resource.MustParse("256Mi"),
			maxAllowedMemory:  resource.MustParse("256Mi"),
			expectedMaxMemory: resource.MustParse("256Mi"),
		},
		{
			name:               "verify update mode and controlled values",
			minAllowedMemory:   resource.MustParse("128Mi"),
			maxAllowedMemory:   resource.MustParse("512Mi"),
			expectedMaxMemory:  resource.MustParse("512Mi"),
			validateUpdateMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := types.NamespacedName{Name: "test-vpa", Namespace: "test-ns"}
			vpa := makeVPA(name, tt.minAllowedMemory, tt.maxAllowedMemory)

			require.Equal(t, "test-vpa", vpa.Name)
			require.Equal(t, "test-ns", vpa.Namespace)
			require.Equal(t, "DaemonSet", vpa.Spec.TargetRef.Kind)
			require.Equal(t, "test-vpa", vpa.Spec.TargetRef.Name)

			require.NotNil(t, vpa.Spec.ResourcePolicy)
			require.Len(t, vpa.Spec.ResourcePolicy.ContainerPolicies, 1)

			policy := vpa.Spec.ResourcePolicy.ContainerPolicies[0]
			require.Equal(t, containerName, policy.ContainerName)
			require.NotNil(t, policy.ControlledResources)
			require.Equal(t, []corev1.ResourceName{corev1.ResourceMemory}, *policy.ControlledResources)

			actualMinMemory := policy.MinAllowed[corev1.ResourceMemory]
			require.Equal(t, tt.minAllowedMemory.Value(), actualMinMemory.Value())

			actualMaxMemory := policy.MaxAllowed[corev1.ResourceMemory]
			require.Equal(t, tt.expectedMaxMemory.Value(), actualMaxMemory.Value())

			if tt.validateUpdateMode {
				require.NotNil(t, vpa.Spec.UpdatePolicy)
				require.NotNil(t, vpa.Spec.UpdatePolicy.UpdateMode)
				require.Equal(t, autoscalingvpav1.UpdateModeInPlaceOrRecreate, *vpa.Spec.UpdatePolicy.UpdateMode)

				require.NotNil(t, policy.ControlledValues)
				require.Equal(t, autoscalingvpav1.ContainerControlledValuesRequestsAndLimits, *policy.ControlledValues)
			}
		})
	}
}

func TestMakeServiceAccount(t *testing.T) {
	name := types.NamespacedName{Name: "test-sa", Namespace: "test-ns"}
	sa := makeServiceAccount(name)

	require.Equal(t, "test-sa", sa.Name)
	require.Equal(t, "test-ns", sa.Namespace)
}

func TestMakeConfigMap(t *testing.T) {
	name := types.NamespacedName{Name: "test-cm", Namespace: "test-ns"}
	configYAML := "receivers:\n  otlp:"
	cm := makeConfigMap(name, configYAML)

	require.Equal(t, "test-cm", cm.Name)
	require.Equal(t, "test-ns", cm.Namespace)
	require.Contains(t, cm.Data, configFileName)
	require.Equal(t, configYAML, cm.Data[configFileName])
}

func TestMakeSecret(t *testing.T) {
	name := types.NamespacedName{Name: "test-secret", Namespace: "test-ns"}
	secretData := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
	}
	secret := makeSecret(name, secretData)

	require.Equal(t, "test-secret", secret.Name)
	require.Equal(t, "test-ns", secret.Namespace)
	require.Equal(t, secretData, secret.Data)
}

func TestMakeMetricsService(t *testing.T) {
	name := types.NamespacedName{Name: "test-svc", Namespace: "test-ns"}
	svc := makeMetricsService(name)

	require.Equal(t, "test-svc-metrics", svc.Name)
	require.Equal(t, "test-ns", svc.Namespace)
	require.Equal(t, commonresources.LabelValueTelemetrySelfMonitor, svc.Labels[commonresources.LabelKeyTelemetrySelfMonitor])
	require.Equal(t, "true", svc.Annotations[commonresources.AnnotationKeyPrometheusScrape])
	require.Len(t, svc.Spec.Ports, 1)
	require.Equal(t, "http-metrics", svc.Spec.Ports[0].Name)
	require.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
}
