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
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kyma-project/telemetry-manager/internal/config"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

func TestApplyCommonResources_ErrorHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	tests := []struct {
		name          string
		rbac          rbac
		expectError   bool
		errorContains string
		setupClient   func() client.Client
	}{
		{
			name: "service account creation fails",
			rbac: rbac{},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*corev1.ServiceAccount); ok {
								return errors.New("service account error")
							}

							return client.Create(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:   true,
			errorContains: "failed to create service account",
		},
		{
			name: "cluster role creation fails",
			rbac: rbac{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cr"},
				},
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*rbacv1.ClusterRole); ok {
								return errors.New("cluster role error")
							}

							return client.Create(ctx, obj, opts...)
						},
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if _, ok := obj.(*rbacv1.ClusterRole); ok {
								return errors.New("cluster role error")
							}

							return client.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:   true,
			errorContains: "failed to create cluster role",
		},
		{
			name: "cluster role binding creation fails",
			rbac: rbac{
				clusterRoleBinding: &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-crb"},
				},
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
								return errors.New("cluster role binding error")
							}

							return client.Create(ctx, obj, opts...)
						},
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if _, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
								return errors.New("cluster role binding error")
							}

							return client.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:   true,
			errorContains: "failed to create cluster role binding",
		},
		{
			name: "role creation fails",
			rbac: rbac{
				role: &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: "test-role", Namespace: "test-ns"},
				},
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*rbacv1.Role); ok {
								return errors.New("role error")
							}

							return client.Create(ctx, obj, opts...)
						},
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if _, ok := obj.(*rbacv1.Role); ok {
								return errors.New("role error")
							}

							return client.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:   true,
			errorContains: "failed to create role",
		},
		{
			name: "role binding creation fails",
			rbac: rbac{
				roleBinding: &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rb", Namespace: "test-ns"},
				},
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*rbacv1.RoleBinding); ok {
								return errors.New("role binding error")
							}

							return client.Create(ctx, obj, opts...)
						},
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if _, ok := obj.(*rbacv1.RoleBinding); ok {
								return errors.New("role binding error")
							}

							return client.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:   true,
			errorContains: "failed to create role binding",
		},
		{
			name: "metrics service creation fails",
			rbac: rbac{},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if svc, ok := obj.(*corev1.Service); ok && svc.Name != "" {
								return errors.New("service error")
							}

							return client.Create(ctx, obj, opts...)
						},
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if svc, ok := obj.(*corev1.Service); ok && svc.Name != "" {
								return errors.New("service error")
							}

							return client.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:   true,
			errorContains: "failed to create metrics service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			name := types.NamespacedName{Name: "test", Namespace: "test-ns"}
			err := applyCommonResources(context.Background(), c, name, tt.rbac)

			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
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

func TestOTLPGateway_ApplyVPA(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

	globals := config.NewGlobal(config.WithTargetNamespace("test-ns"))
	sut := NewOTLPGatewayApplierDeleter(globals, "test-image", "normal")

	tests := []struct {
		name        string
		opts        GatewayApplyOptions
		wantErr     bool
		errContains string
		setupClient func() client.Client
	}{
		{
			name: "VPA CRD does not exist",
			opts: GatewayApplyOptions{
				VpaCRDExists: false,
				VpaEnabled:   true,
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
		},
		{
			name: "VPA enabled - creates VPA",
			opts: GatewayApplyOptions{
				VpaCRDExists:        true,
				VpaEnabled:          true,
				VPAMaxAllowedMemory: resource.MustParse("1Gi"),
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
		},
		{
			name: "VPA disabled - deletes VPA",
			opts: GatewayApplyOptions{
				VpaCRDExists: true,
				VpaEnabled:   false,
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
		},
		{
			name: "VPA creation fails",
			opts: GatewayApplyOptions{
				VpaCRDExists:        true,
				VpaEnabled:          true,
				VPAMaxAllowedMemory: resource.MustParse("1Gi"),
			},
			wantErr:     true,
			errContains: "failed to create VPA",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*autoscalingvpav1.VerticalPodAutoscaler); ok {
								return errors.New("vpa error")
							}

							return client.Create(ctx, obj, opts...)
						},
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if _, ok := obj.(*autoscalingvpav1.VerticalPodAutoscaler); ok {
								return errors.New("vpa error")
							}

							return client.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
		},
		{
			name: "VPA deletion fails",
			opts: GatewayApplyOptions{
				VpaCRDExists: true,
				VpaEnabled:   false,
			},
			wantErr:     true,
			errContains: "failed to delete VPA",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
							if _, ok := obj.(*autoscalingvpav1.VerticalPodAutoscaler); ok {
								return errors.New("vpa error")
							}

							return client.Delete(ctx, obj, opts...)
						},
					}).
					Build()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			namespacedName := types.NamespacedName{
				Name:      sut.baseName,
				Namespace: sut.globals.TargetNamespace(),
			}
			err := sut.applyVPA(context.Background(), c, c, namespacedName, tt.opts)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOTLPGateway_MakeGatewayResourceRequirements(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("test-ns"))
	sut := NewOTLPGatewayApplierDeleter(globals, "test-image", "normal")

	tests := []struct {
		name           string
		opts           GatewayApplyOptions
		validateMemory func(t *testing.T, resources corev1.ResourceRequirements)
		validateCPU    func(t *testing.T, resources corev1.ResourceRequirements)
	}{
		{
			name: "base resources without multiplier",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 0,
			},
			validateMemory: func(t *testing.T, resources corev1.ResourceRequirements) {
				require.True(t, sut.baseMemoryRequest.Equal(resources.Requests[corev1.ResourceMemory]))
				require.True(t, sut.baseMemoryLimit.Equal(resources.Limits[corev1.ResourceMemory]))
			},
		},
		{
			name: "resources with multiplier of 3",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 3,
			},
			validateCPU: func(t *testing.T, resources corev1.ResourceRequirements) {
				expectedCPURequest := sut.baseCPURequest.DeepCopy()
				for range 3 {
					expectedCPURequest.Add(sut.dynamicCPURequest)
				}

				require.True(t, expectedCPURequest.Equal(resources.Requests[corev1.ResourceCPU]))
			},
		},
		{
			name: "VPA enabled - memory limit is 2x request",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 2,
				VpaCRDExists:                   true,
				VpaEnabled:                     true,
			},
			validateMemory: func(t *testing.T, resources corev1.ResourceRequirements) {
				memoryRequest := resources.Requests[corev1.ResourceMemory]
				memoryLimit := resources.Limits[corev1.ResourceMemory]
				expectedLimit := memoryRequest.DeepCopy()
				expectedLimit.Add(memoryRequest)
				require.True(t, expectedLimit.Equal(memoryLimit))
			},
		},
		{
			name: "VPA disabled - uses calculated memory limit",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 1,
				VpaCRDExists:                   false,
				VpaEnabled:                     false,
			},
			validateMemory: func(t *testing.T, resources corev1.ResourceRequirements) {
				expectedMemoryLimit := sut.baseMemoryLimit.DeepCopy()
				expectedMemoryLimit.Add(sut.dynamicMemoryLimit)
				require.True(t, expectedMemoryLimit.Equal(resources.Limits[corev1.ResourceMemory]))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := sut.makeGatewayResourceRequirements(tt.opts)

			if tt.validateMemory != nil {
				tt.validateMemory(t, resources)
			}

			if tt.validateCPU != nil {
				tt.validateCPU(t, resources)
			}
		})
	}
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

func TestMakeServiceAccount(t *testing.T) {
	name := types.NamespacedName{Name: "test-sa", Namespace: "test-ns"}
	sa := makeServiceAccount(name)

	require.Equal(t, "test-sa", sa.Name)
	require.Equal(t, "test-ns", sa.Namespace)
}

func TestMakeMetricsService(t *testing.T) {
	name := types.NamespacedName{Name: "test-svc", Namespace: "test-ns"}
	svc := makeMetricsService(name)

	expectedName := names.MetricsServiceName("test-svc")
	require.Equal(t, expectedName, svc.Name)
	require.Equal(t, "test-ns", svc.Namespace)
	require.Equal(t, commonresources.LabelValueTelemetrySelfMonitor, svc.Labels[commonresources.LabelKeyTelemetrySelfMonitor])
	require.Equal(t, "true", svc.Annotations[commonresources.AnnotationKeyPrometheusScrape])
	require.Len(t, svc.Spec.Ports, 1)
	require.Equal(t, "http-metrics", svc.Spec.Ports[0].Name)
	require.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
}
