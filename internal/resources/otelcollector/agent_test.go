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

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestAgent_ApplyResources(t *testing.T) {
	var objects []client.Object

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			objects = append(objects, obj)
			// Nothing has to be created, just add created object to the list
			return nil
		},
	}).Build()

	image := "opentelemetry/collector:dummy"
	namespace := "kyma-system"
	priorityClassName := "normal"
	sut := NewMetricAgentApplierDeleter(image, namespace, priorityClassName)

	err := sut.ApplyResources(context.Background(), client, AgentApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: "dummy",
	})
	require.NoError(t, err)

	bytes, err := testutils.MarshalYAML(scheme, objects)
	require.NoError(t, err)

	goldenFileBytes, err := os.ReadFile("testdata/metric-agent.yaml")
	require.NoError(t, err)

	require.Equal(t, string(goldenFileBytes), string(bytes))
}

func TestAgent_DeleteResources(t *testing.T) {
	var created []client.Object

	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			created = append(created, obj)
			return c.Create(ctx, obj)
		},
	}).Build()

	image := "opentelemetry/collector:dummy"
	namespace := "kyma-system"
	priorityClassName := "normal"
	sut := NewMetricAgentApplierDeleter(image, namespace, priorityClassName)

	err := sut.ApplyResources(context.Background(), fakeClient, AgentApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: "dummy",
	})
	require.NoError(t, err)

	err = sut.DeleteResources(context.Background(), fakeClient)
	require.NoError(t, err)

	for i := range created {
		// an update operation on a non-existent object should return a NotFound error
		err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(created[i]), created[i])
		require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
	}
}
