package otelcollector

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kyma-project/telemetry-manager/internal/config"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestOTLPGateway_ApplyResources(t *testing.T) {
	globals := config.NewGlobal(
		config.WithTargetNamespace("kyma-system"),
		config.WithImagePullSecretName("mySecret"),
		config.WithAdditionalWorkloadLabels(map[string]string{"test-label-key": "test-label-value"}),
		config.WithAdditionalWorkloadAnnotations(map[string]string{"test-anno-key": "test-anno-value"}),
		config.WithClusterTrustBundleName("trustBundle"),
	)
	globalsWithFIPS := config.NewGlobal(
		config.WithTargetNamespace("kyma-system"),
		config.WithOperateInFIPSMode(true),
	)
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	// Interface for testing both gateway types
	type gatewayApplierDeleter interface {
		ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error
		DeleteResources(ctx context.Context, c client.Client, isIstioActive bool, vpaCRDExists bool) error
	}

	tests := []struct {
		name                           string
		sut                            gatewayApplierDeleter
		istioEnabled                   bool
		vpaCRDExists                   bool
		vpaEnabled                     bool
		vpaMaxAllowedMemory            resource.Quantity
		goldenFilePath                 string
		resourceRequirementsMultiplier int
	}{
		{
			name:           "OTLP Gateway",
			sut:            NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath: "testdata/otlp-gateway.yaml",
		},
		{
			name:           "OTLP Gateway with istio",
			sut:            NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled:   true,
			goldenFilePath: "testdata/otlp-gateway-istio.yaml",
		},
		{
			name:           "OTLP Gateway with FIPS mode enabled",
			sut:            NewOTLPGatewayApplierDeleter(globalsWithFIPS, image, priorityClassName),
			goldenFilePath: "testdata/otlp-gateway-fips-enabled.yaml",
		},
		{
			name:                "OTLP gateway with VPA",
			sut:                 NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath:      "testdata/otlp-gateway-vpa.yaml",
			vpaCRDExists:        true,
			vpaEnabled:          true,
			vpaMaxAllowedMemory: resource.MustParse("1Gi"),
		},
		{
			name:                "OTLP gateway with VPA and zero max allowed memory",
			sut:                 NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath:      "testdata/otlp-gateway-vpa-zero-max-memory.yaml",
			vpaCRDExists:        true,
			vpaEnabled:          true,
			vpaMaxAllowedMemory: resource.Quantity{}, // zero, must be clamped to min allowed memory
		},
		{
			name:                           "OTLP gateway multi instance with VPA",
			sut:                            NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath:                 "testdata/otlp-multi-instance-gateway-vpa.yaml",
			vpaCRDExists:                   true,
			vpaEnabled:                     true,
			vpaMaxAllowedMemory:            resource.MustParse("1Gi"),
			resourceRequirementsMultiplier: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object

			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istionetworkingclientv1.AddToScheme(scheme))
			utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
			utilruntime.Must(v1alpha3.AddToScheme(scheme))
			utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					objects = append(objects, obj)
					// Nothing has to be created, just add created object to the list
					return nil
				},
			}).Build()

			err := tt.sut.ApplyResources(t.Context(), fakeClient, GatewayApplyOptions{
				CollectorConfigYAML: "dummy",
				CollectorEnvVars: map[string][]byte{
					"DUMMY_ENV_VAR": []byte("foo"),
				},
				IstioEnabled:                   tt.istioEnabled,
				Replicas:                       2,
				VpaCRDExists:                   tt.vpaCRDExists,
				VpaEnabled:                     tt.vpaEnabled,
				VPAMaxAllowedMemory:            tt.vpaMaxAllowedMemory,
				ResourceRequirementsMultiplier: tt.resourceRequirementsMultiplier,
			})
			require.NoError(t, err)

			bytes, err := testutils.MarshalYAML(scheme, objects)
			require.NoError(t, err)

			if testutils.ShouldUpdateGoldenFiles() {
				testutils.UpdateGoldenFileYAML(t, tt.goldenFilePath, bytes)
				return
			}

			goldenFileBytes, err := os.ReadFile(tt.goldenFilePath)
			require.NoError(t, err)

			require.Equal(t, string(goldenFileBytes), string(bytes))
		})
	}
}

func TestOTLPGateway_DeleteResources(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	// Interface for testing both gateway types
	type gatewayApplierDeleter interface {
		ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error
		DeleteResources(ctx context.Context, c client.Client, isIstioActive bool, vpaCRDExists bool) error
	}

	tests := []struct {
		name         string
		sut          gatewayApplierDeleter
		istioEnabled bool
	}{

		{
			name: "OTLP Gateway",
			sut:  NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
		},
		{
			name:         "OTLP Gateway  with istio",
			sut:          NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var created []client.Object

			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
			utilruntime.Must(istionetworkingclientv1.AddToScheme(scheme))
			utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					created = append(created, obj)
					return c.Create(ctx, obj)
				},
			}).Build()

			err := tt.sut.ApplyResources(t.Context(), fakeClient, GatewayApplyOptions{
				IstioEnabled: tt.istioEnabled,
				VpaCRDExists: true,
				VpaEnabled:   true,
			})
			require.NoError(t, err)

			err = tt.sut.DeleteResources(t.Context(), fakeClient, tt.istioEnabled, true)
			require.NoError(t, err)

			for i := range created {
				// All objects should be deleted
				err = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(created[i]), created[i])
				require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
			}
		})
	}
}

func TestOTLPGateway_Annotations(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name                        string
		sut                         *OTLPGatewayApplierDeleter
		opts                        GatewayApplyOptions
		expectedIncludeInboundPorts string
		shouldHaveInterceptionMode  bool
	}{
		{
			name: "OTLP Gateway without istio",
			sut:  NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			opts: GatewayApplyOptions{
				IstioEnabled: false,
			},
			shouldHaveInterceptionMode: false,
		},
		{
			name: "OTLP Gateway with istio - metrics, grpc, and http ports excluded",
			sut:  NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			opts: GatewayApplyOptions{
				IstioEnabled: true,
			},
			expectedIncludeInboundPorts: "",
			shouldHaveInterceptionMode:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := tt.sut.makeAnnotations("dummy-checksum", tt.opts)

			if !tt.opts.IstioEnabled {
				require.NotContains(t, annotations, "traffic.sidecar.istio.io/includeInboundPorts")
				require.NotContains(t, annotations, "sidecar.istio.io/interceptionMode")

				return
			}

			require.Equal(t, tt.expectedIncludeInboundPorts, annotations["traffic.sidecar.istio.io/includeInboundPorts"])

			if tt.shouldHaveInterceptionMode {
				require.Equal(t, "TPROXY", annotations["sidecar.istio.io/interceptionMode"])
			} else {
				require.NotContains(t, annotations, "sidecar.istio.io/interceptionMode")
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
		{
			name: "VPA disabled with cap applied - memory limit capped at 2x VPAMaxAllowedMemory",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 10, // Would result in very high memory
				VpaCRDExists:                   true,
				VpaEnabled:                     false,
				VPAMaxAllowedMemory:            resource.MustParse("1Gi"),
			},
			validateMemory: func(t *testing.T, resources corev1.ResourceRequirements) {
				// With 10 pipelines: 500Mi + (10 × 1500Mi) = 15.5Gi
				// But capped at 2 × 1Gi = 2Gi
				maxCap := resource.MustParse("2Gi")
				memoryLimit := resources.Limits[corev1.ResourceMemory]
				require.True(t, memoryLimit.Cmp(maxCap) <= 0, "Memory limit %s should not exceed cap %s", memoryLimit.String(), maxCap.String())
				require.True(t, maxCap.Equal(memoryLimit), "Memory limit should be capped at 2Gi")
			},
		},
		{
			name: "VPA disabled with cap not applied - calculated limit below cap",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 1,
				VpaCRDExists:                   true,
				VpaEnabled:                     false,
				VPAMaxAllowedMemory:            resource.MustParse("10Gi"), // Very high cap
			},
			validateMemory: func(t *testing.T, resources corev1.ResourceRequirements) {
				// With 1 pipeline: 500Mi + 1500Mi = 2000Mi
				// This is well below 2×10Gi cap, so uses calculated value
				expectedMemoryLimit := sut.baseMemoryLimit.DeepCopy()
				expectedMemoryLimit.Add(sut.dynamicMemoryLimit)

				memoryLimit := resources.Limits[corev1.ResourceMemory]
				require.True(t, expectedMemoryLimit.Equal(memoryLimit))
			},
		},
		{
			name: "VPA disabled with zero max memory - no capping applied",
			opts: GatewayApplyOptions{
				ResourceRequirementsMultiplier: 3,
				VpaCRDExists:                   true,
				VpaEnabled:                     false,
				VPAMaxAllowedMemory:            resource.Quantity{}, // Zero value
			},
			validateMemory: func(t *testing.T, resources corev1.ResourceRequirements) {
				// Zero VPAMaxAllowedMemory means no capping
				// Should use calculated limit: 500Mi + (3 × 1500Mi) = 5000Mi
				expectedMemoryLimit := sut.baseMemoryLimit.DeepCopy()
				for range 3 {
					expectedMemoryLimit.Add(sut.dynamicMemoryLimit)
				}

				memoryLimit := resources.Limits[corev1.ResourceMemory]
				require.True(t, expectedMemoryLimit.Equal(memoryLimit))
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
