package otelcollector

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestGateway_ApplyResources(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	globalsWithFIPS := config.NewGlobal(
		config.WithTargetNamespace("kyma-system"),
		config.WithOperateInFIPSMode(true),
	)
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name           string
		sut            *GatewayApplierDeleter
		istioEnabled   bool
		goldenFilePath string
	}{
		{
			name:           "metric gateway",
			sut:            NewMetricGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath: "testdata/metric-gateway.yaml",
		},
		{
			name:           "metric gateway with istio",
			sut:            NewMetricGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled:   true,
			goldenFilePath: "testdata/metric-gateway-istio.yaml",
		},
		{
			name:           "metric gateway with FIPS mode enabled",
			sut:            NewMetricGatewayApplierDeleter(globalsWithFIPS, image, priorityClassName),
			goldenFilePath: "testdata/metric-gateway-fips-enabled.yaml",
		},
		{
			name:           "trace gateway",
			sut:            NewTraceGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath: "testdata/trace-gateway.yaml",
		},
		{
			name:           "trace gateway with istio",
			sut:            NewTraceGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled:   true,
			goldenFilePath: "testdata/trace-gateway-istio.yaml",
		},
		{
			name:           "trace gateway with FIPS mode enabled",
			sut:            NewTraceGatewayApplierDeleter(globalsWithFIPS, image, priorityClassName),
			goldenFilePath: "testdata/trace-gateway-fips-enabled.yaml",
		},
		{
			name:           "log gateway",
			sut:            NewLogGatewayApplierDeleter(globals, image, priorityClassName),
			goldenFilePath: "testdata/log-gateway.yaml",
		},
		{
			name:           "log gateway with istio",
			sut:            NewLogGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled:   true,
			goldenFilePath: "testdata/log-gateway-istio.yaml",
		},
		{
			name:           "log gateway with FIPS mode enabled",
			sut:            NewLogGatewayApplierDeleter(globalsWithFIPS, image, priorityClassName),
			goldenFilePath: "testdata/log-gateway-fips-enabled.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object

			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))

			client := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					objects = append(objects, obj)
					// Nothing has to be created, just add created object to the list
					return nil
				},
			}).Build()

			err := tt.sut.ApplyResources(t.Context(), client, GatewayApplyOptions{
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

func TestGateway_DeleteResources(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name         string
		sut          *GatewayApplierDeleter
		istioEnabled bool
	}{
		{
			name: "metric gateway",
			sut:  NewMetricGatewayApplierDeleter(globals, image, priorityClassName),
		},
		{
			name:         "metric gateway with istio",
			sut:          NewMetricGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled: true,
		},
		{
			name: "trace gateway",
			sut:  NewTraceGatewayApplierDeleter(globals, image, priorityClassName),
		},
		{
			name:         "trace gateway with istio",
			sut:          NewTraceGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled: true,
		},
		{
			name: "log gateway",
			sut:  NewLogGatewayApplierDeleter(globals, image, priorityClassName),
		},
		{
			name:         "log gateway with istio",
			sut:          NewLogGatewayApplierDeleter(globals, image, priorityClassName),
			istioEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var created []client.Object

			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))

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
				// an update operation on a non-existent object should return a NotFound error
				err = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(created[i]), created[i])
				require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
			}
		})
	}
}
