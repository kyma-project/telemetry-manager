package otelcollector

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		name           string
		sut            gatewayApplierDeleter
		istioEnabled   bool
		goldenFilePath string
	}{
		{
			name:           "OTLP gateway",
			sut:            NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath: "testdata/otlp-gateway.yaml",
		},
		{
			name:           "OTLP gateway with istio",
			sut:            NewOTLPGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled:   true,
			goldenFilePath: "testdata/otlp-gateway-istio.yaml",
		},
		{
			name:           "OTLP gateway with FIPS mode enabled",
			sut:            NewOTLPGatewayApplierDeleter(globalsWithFIPS, image, priorityClassName),
			goldenFilePath: "testdata/otlp-gateway-fips-enabled.yaml",
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
				IstioEnabled: tt.istioEnabled,
				Replicas:     2,
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
			name:         "OTLP gateway  with istio",
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

func TestOTLPGateway_VPA(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name            string
		vpaCRDExists    bool
		vpaEnabled      bool
		expectVPACreate bool
		expectVPADelete bool
	}{
		{
			name:            "VPA enabled and CRD exists",
			vpaCRDExists:    true,
			vpaEnabled:      true,
			expectVPACreate: true,
			expectVPADelete: false,
		},
		{
			name:            "VPA disabled but CRD exists",
			vpaCRDExists:    true,
			vpaEnabled:      false,
			expectVPACreate: false,
			expectVPADelete: true,
		},
		{
			name:            "VPA enabled but CRD does not exist",
			vpaCRDExists:    false,
			vpaEnabled:      true,
			expectVPACreate: false,
			expectVPADelete: false,
		},
		{
			name:            "VPA disabled and CRD does not exist",
			vpaCRDExists:    false,
			vpaEnabled:      false,
			expectVPACreate: false,
			expectVPADelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

			var (
				createdVPA bool
				deletedVPA bool
			)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					if _, ok := obj.(*autoscalingvpav1.VerticalPodAutoscaler); ok {
						createdVPA = true
					}

					return nil
				},
				Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
					if _, ok := obj.(*autoscalingvpav1.VerticalPodAutoscaler); ok {
						deletedVPA = true
					}

					return apierrors.NewNotFound(schema.GroupResource{}, "")
				},
			}).Build()

			sut := NewOTLPGatewayApplierDeleter(globals, image, priorityClassName)
			err := sut.ApplyResources(t.Context(), fakeClient, GatewayApplyOptions{
				VpaCRDExists: tt.vpaCRDExists,
				VpaEnabled:   tt.vpaEnabled,
			})
			require.NoError(t, err)

			require.Equal(t, tt.expectVPACreate, createdVPA, "VPA create expectation mismatch")
			require.Equal(t, tt.expectVPADelete, deletedVPA, "VPA delete expectation mismatch")
		})
	}
}

func TestOTLPGateway_ResourceRequirements(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name                           string
		resourceRequirementsMultiplier int
		vpaCRDExists                   bool
		vpaEnabled                     bool
	}{
		{
			name:                           "no multiplier, no VPA",
			resourceRequirementsMultiplier: 0,
			vpaCRDExists:                   false,
			vpaEnabled:                     false,
		},
		{
			name:                           "with multiplier, no VPA",
			resourceRequirementsMultiplier: 3,
			vpaCRDExists:                   false,
			vpaEnabled:                     false,
		},
		{
			name:                           "no multiplier, with VPA",
			resourceRequirementsMultiplier: 0,
			vpaCRDExists:                   true,
			vpaEnabled:                     true,
		},
		{
			name:                           "with multiplier and VPA",
			resourceRequirementsMultiplier: 2,
			vpaCRDExists:                   true,
			vpaEnabled:                     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sut := NewOTLPGatewayApplierDeleter(globals, image, priorityClassName)

			resources := sut.makeGatewayResourceRequirements(GatewayApplyOptions{
				ResourceRequirementsMultiplier: tt.resourceRequirementsMultiplier,
				VpaCRDExists:                   tt.vpaCRDExists,
				VpaEnabled:                     tt.vpaEnabled,
			})

			require.NotNil(t, resources.Requests)
			require.NotNil(t, resources.Limits)
			require.NotEmpty(t, resources.Requests[corev1.ResourceCPU])
			require.NotEmpty(t, resources.Requests[corev1.ResourceMemory])
			require.NotEmpty(t, resources.Limits[corev1.ResourceMemory])

			// When VPA is enabled, memory limit should be 2x memory request
			if tt.vpaCRDExists && tt.vpaEnabled {
				memRequest := resources.Requests[corev1.ResourceMemory]
				memLimit := resources.Limits[corev1.ResourceMemory]
				expectedLimit := memRequest.DeepCopy()
				expectedLimit.Add(memRequest)
				require.Equal(t, expectedLimit.String(), memLimit.String())
			}
		})
	}
}

func TestOTLPGateway_MakeGatewayMetadata(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name         string
		istioEnabled bool
	}{
		{
			name:         "without istio",
			istioEnabled: false,
		},
		{
			name:         "with istio",
			istioEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sut := NewOTLPGatewayApplierDeleter(globals, image, priorityClassName)

			metadata := sut.makeGatewayMetadata("test-checksum", GatewayApplyOptions{
				IstioEnabled:                   tt.istioEnabled,
				ResourceRequirementsMultiplier: 1,
			})

			require.NotNil(t, metadata.ResourceLabels)
			require.NotNil(t, metadata.PodLabels)
			require.NotNil(t, metadata.PodAnnotations)
			require.Equal(t, "test-checksum", metadata.PodAnnotations["checksum/config"])

			if tt.istioEnabled {
				require.Equal(t, "", metadata.PodAnnotations["traffic.sidecar.istio.io/includeInboundPorts"])
				require.Equal(t, "TPROXY", metadata.PodAnnotations["sidecar.istio.io/interceptionMode"])
			}
		})
	}
}
