package otelcollector

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
		config.WithAdditionalLabels(map[string]string{"test-label-key": "test-label-value"}),
		config.WithAdditionalAnnotations(map[string]string{"test-anno-key": "test-anno-value"}),
		config.WithClusterTrustBundleName("trustBundle"),
	)
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	// Interface for testing both gateway types
	type gatewayApplierDeleter interface {
		ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error
		DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object

			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istionetworkingclientv1.AddToScheme(scheme))
			utilruntime.Must(v1alpha3.AddToScheme(scheme))

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
		DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
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

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					created = append(created, obj)
					return c.Create(ctx, obj)
				},
			}).Build()

			err := tt.sut.ApplyResources(t.Context(), fakeClient, GatewayApplyOptions{
				IstioEnabled: tt.istioEnabled,
			})
			require.NoError(t, err)

			err = tt.sut.DeleteResources(t.Context(), fakeClient, tt.istioEnabled)
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
