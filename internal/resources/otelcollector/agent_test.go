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
	image := "opentelemetry/collector:dummy"
	namespace := "kyma-system"
	priorityClassName := "normal"

	tests := []struct {
		name           string
		sut            *AgentApplierDeleter
		goldenFilePath string
		saveGoldenFile bool
	}{
		{
			name:           "metric agent",
			sut:            NewMetricAgentApplierDeleter(image, namespace, priorityClassName),
			goldenFilePath: "testdata/metric-agent.yaml",
		},
		{
			name:           "log agent",
			sut:            NewLogAgentApplierDeleter(image, namespace, priorityClassName),
			goldenFilePath: "testdata/log-agent.yaml",
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
				AllowedPorts:        []int32{5555, 6666},
				CollectorConfigYAML: "dummy",
			})
			require.NoError(t, err)

			if tt.saveGoldenFile {
				testutils.SaveAsYAML(t, scheme, objects, tt.goldenFilePath)
			}

			bytes, err := testutils.MarshalYAML(scheme, objects)
			require.NoError(t, err)

			goldenFileBytes, err := os.ReadFile(tt.goldenFilePath)
			require.NoError(t, err)

			require.Equal(t, string(goldenFileBytes), string(bytes))
		})
	}
}

func TestAgent_DeleteResources(t *testing.T) {
	image := "opentelemetry/collector:dummy"
	namespace := "kyma-system"
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
			sut:  NewMetricAgentApplierDeleter(image, namespace, priorityClassName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(t.Context(), fakeClient, AgentApplyOptions{
				AllowedPorts:        []int32{5555, 6666},
				CollectorConfigYAML: "dummy",
			})
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
