package otelcollector

import (
	"context"
	"os"
	"testing"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMetricGateway_ApplyResources(t *testing.T) {
	var objects []client.Object
	client := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			objects = append(objects, obj)
			return nil
		},
	}).Build()

	image := "opentelemetry/collector:latest"
	namespace := "kyma-system"
	priorityClassName := "normal"
	sut := NewMetricGatewayApplierDeleter(image, namespace, priorityClassName)

	err := sut.ApplyResources(context.Background(), client, GatewayApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: "dummy",
		CollectorEnvVars: map[string][]byte{
			"DUMMY_ENV_VAR": []byte("foo"),
		},
		Replicas: 2,
	})

	bytes, err := testutils.MarshalYAML(objects)
	require.NoError(t, err)

	goldenFileBytes, err := os.ReadFile("testdata/metric-agent.yaml")
	require.NoError(t, err)

	require.Equal(t, string(goldenFileBytes), string(bytes))
}
