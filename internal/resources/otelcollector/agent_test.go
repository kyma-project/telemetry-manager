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

func TestAgent_ApplyResources(t *testing.T) {
	globals := config.NewGlobal(
		config.WithTargetNamespace("kyma-system"),
		config.WithImagePullSecretName("mySecret"),
		config.WithAdditionalLabels(map[string]string{"test-label-key": "test-label-value"}),
		config.WithAdditionalAnnotations(map[string]string{"test-anno-key": "test-anno-value"}),
		config.WithClusterTrustBundleName("trustBundle"),
	)
	globalsWithFIPS := config.NewGlobal(
		config.WithTargetNamespace("kyma-system"),
		config.WithOperateInFIPSMode(true),
	)
	collectorImage := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	tests := []struct {
		name             string
		sut              *AgentApplierDeleter
		collectorEnvVars map[string][]byte
		istioEnabled     bool
		backendPorts     []string
		goldenFilePath   string
	}{
		{
			name:           "metric agent",
			sut:            NewMetricAgentApplierDeleter(globals, collectorImage, priorityClassName),
			goldenFilePath: "testdata/metric-agent.yaml",
		},
		{
			name:           "metric agent with istio",
			sut:            NewMetricAgentApplierDeleter(globals, collectorImage, priorityClassName),
			istioEnabled:   true,
			backendPorts:   []string{"4317", "9090"},
			goldenFilePath: "testdata/metric-agent-istio.yaml",
		},
		{
			name:           "metric agent with FIPS mode enabled",
			sut:            NewMetricAgentApplierDeleter(globalsWithFIPS, collectorImage, priorityClassName),
			goldenFilePath: "testdata/metric-agent-fips-enabled.yaml",
		},
		{
			name: "log agent",
			sut:  NewLogAgentApplierDeleter(globals, collectorImage, priorityClassName),
			collectorEnvVars: map[string][]byte{
				"DUMMY_ENV_VAR": []byte("foo"),
			},
			goldenFilePath: "testdata/log-agent.yaml",
		},
		{
			name: "log agent with istio",
			sut:  NewLogAgentApplierDeleter(globals, collectorImage, priorityClassName),
			collectorEnvVars: map[string][]byte{
				"DUMMY_ENV_VAR": []byte("foo"),
			},
			istioEnabled:   true,
			goldenFilePath: "testdata/log-agent-istio.yaml",
		},
		{
			name: "log agent with FIPS mode enabled",
			sut:  NewLogAgentApplierDeleter(globalsWithFIPS, collectorImage, priorityClassName),
			collectorEnvVars: map[string][]byte{
				"DUMMY_ENV_VAR": []byte("foo"),
			},
			goldenFilePath: "testdata/log-agent-fips-enabled.yaml",
		},
	}

	for _, tt := range tests {
		var objects []client.Object

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))

		fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
			Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
				objects = append(objects, obj)
				// Nothing has to be created, just add created object to the list
				return nil
			},
		}).Build()

		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(t.Context(), fakeClient, AgentApplyOptions{
				IstioEnabled:        tt.istioEnabled,
				CollectorConfigYAML: "dummy",
				CollectorEnvVars:    tt.collectorEnvVars,
				BackendPorts:        tt.backendPorts,
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

func TestAgent_DeleteResources(t *testing.T) {
	globals := config.NewGlobal(config.WithTargetNamespace("kyma-system"))
	image := "opentelemetry/collector:dummy"
	priorityClassName := "normal"

	var created []client.Object

	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			created = append(created, obj)
			return c.Create(ctx, obj)
		},
	}).Build()

	tests := []struct {
		name string
		sut  *AgentApplierDeleter
	}{
		{
			name: "metric agent",
			sut:  NewMetricAgentApplierDeleter(globals, image, priorityClassName),
		},
		{
			name: "log agent",
			sut:  NewLogAgentApplierDeleter(globals, image, priorityClassName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(t.Context(), fakeClient, AgentApplyOptions{})
			require.NoError(t, err)

			err = tt.sut.DeleteResources(t.Context(), fakeClient)
			require.NoError(t, err)

			for i := range created {
				// an update operation on a non-existent object should return a NotFound error
				err = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(created[i]), created[i])
				require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
			}
		})
	}
}
