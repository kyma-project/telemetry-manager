package selfmonitor

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

const (
	namespace            = "my-namespace"
	name                 = "my-self-monitor"
	prometheusConfigYAML = "dummy prometheus Config"
	alertRulesYAML       = "dummy alert rules"
	configPath           = "/dummy/"
	configFileName       = "dummy-config.yaml"
	alertRulesFileName   = "dummy-alerts.yaml"
)

func TestApplySelfMonitorResources(t *testing.T) {
	var objects []client.Object

	ctx := context.Background()
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			objects = append(objects, obj)
			// Nothing has to be created, just add created object to the list
			return nil
		},
	}).Build()
	sut := ApplierDeleter{
		Config: Config{
			BaseName:  name,
			Namespace: namespace,
		},
	}

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	opts := ApplyOptions{
		AlertRulesFileName:       alertRulesFileName,
		AlertRulesYAML:           alertRulesYAML,
		PrometheusConfigFileName: configFileName,
		PrometheusConfigPath:     configPath,
		PrometheusConfigYAML:     prometheusConfigYAML,
	}
	err := sut.ApplyResources(ctx, client, opts)
	require.NoError(t, err)

	// uncomment to re-generate golden file
	// testutils.SaveAsYAML(t, scheme, objects, "testdata/self-monitor.yaml")

	bytes, err := testutils.MarshalYAML(scheme, objects)
	require.NoError(t, err)

	goldenFileBytes, err := os.ReadFile("testdata/self-monitor.yaml")
	require.NoError(t, err)

	require.Equal(t, string(goldenFileBytes), string(bytes))
}

func TestDeleteSelfMonitorResources(t *testing.T) {
	var created []client.Object

	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			created = append(created, obj)
			return c.Create(ctx, obj)
		},
	}).Build()

	sut := ApplierDeleter{
		Config: Config{
			BaseName:  name,
			Namespace: namespace,
		},
	}

	opts := ApplyOptions{
		AlertRulesFileName:       alertRulesFileName,
		AlertRulesYAML:           alertRulesYAML,
		PrometheusConfigFileName: configFileName,
		PrometheusConfigPath:     configPath,
		PrometheusConfigYAML:     prometheusConfigYAML,
	}
	err := sut.ApplyResources(context.Background(), fakeClient, opts)
	require.NoError(t, err)

	err = sut.DeleteResources(context.Background(), fakeClient)
	require.NoError(t, err)

	for i := range created {
		// an update operation on a non-existent object should return a NotFound error
		err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(created[i]), created[i])
		require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
	}
}
