package otelcollector

import (
	"context"
	"os"
	"testing"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMetricGateway_ApplyResources(t *testing.T) {
	var objects []client.Object
	client := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			objects = append(objects, obj)
			// Nothing has to be created, just add created object to the list
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

	goldenFileBytes, err := os.ReadFile("testdata/metric-gateway.yaml")
	require.NoError(t, err)

	require.Equal(t, string(goldenFileBytes), string(bytes))
}

func TestMetricGateway_DeleteResources(t *testing.T) {
	var created []client.Object
	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			created = append(created, obj)
			return c.Create(context.Background(), obj)
		},
	}).Build()

	image := "opentelemetry/collector:latest"
	namespace := "kyma-system"
	priorityClassName := "normal"
	sut := NewMetricGatewayApplierDeleter(image, namespace, priorityClassName)

	err := sut.ApplyResources(context.Background(), fakeClient, GatewayApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: "dummy",
	})
	require.NoError(t, err)

	err = sut.DeleteResources(context.Background(), fakeClient, false)
	require.NoError(t, err)

	for i := range created {
		// an update operation on a non-existent object should return a NotFound error
		err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(created[i]), created[i])
		require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
	}
}
