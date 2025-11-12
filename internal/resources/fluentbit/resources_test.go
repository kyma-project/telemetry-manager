package fluentbit

import (
	"context"
	"os"
	"testing"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestAgent_ApplyResources(t *testing.T) {
	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	initContainerImage := "alpine"
	priorityClassName := "foo-prio-class"
	namespace := "kyma-system"

	tests := []struct {
		name            string
		sut             *AgentApplierDeleter
		goldenFilePath  string
		saveGoldenFile  bool
		imagePullSecret bool
	}{
		{
			name:           "fluentbit",
			sut:            NewFluentBitApplierDeleter(namespace, image, exporterImage, initContainerImage, priorityClassName),
			goldenFilePath: "testdata/fluentbit.yaml",
		},
		{
			name:            "fluentbit with image pull secret",
			sut:             NewFluentBitApplierDeleter(namespace, image, exporterImage, initContainerImage, priorityClassName),
			goldenFilePath:  "testdata/fluentbit-img-pull-secret.yaml",
			imagePullSecret: true,
		},
	}

	for _, tt := range tests {
		var objects []client.Object

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
		utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
				objects = append(objects, obj)
				// Nothing has to be created, just add created object to the list
				return nil
			},
		}).Build()

		if tt.imagePullSecret {
			t.Setenv(commonresources.ImagePullSecretName, "foo-secret")
		}

		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(t.Context(), fakeClient, AgentApplyOptions{
				AllowedPorts: []int32{5555, 6666},
				FluentBitConfig: &builder.FluentBitConfig{
					SectionsConfig:  map[string]string{"pipeline1.conf": "dummy-sections-content"},
					FilesConfig:     map[string]string{"file1": "dummy-file-content"},
					EnvConfigSecret: map[string][]byte{"env-config-secret1": []byte("dummy-value")},
					TLSConfigSecret: map[string][]byte{"tls-config-secret1": []byte("dummy-value")},
				},
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
	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	initContainerImage := "alpine"
	priorityClassName := "foo-prio-class"
	namespace := "kyma-system"

	var created []client.Object

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
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
			name: "fluentbit",
			sut:  NewFluentBitApplierDeleter(namespace, image, exporterImage, initContainerImage, priorityClassName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentApplyOptions := AgentApplyOptions{
				AllowedPorts:    []int32{5555, 6666},
				FluentBitConfig: &builder.FluentBitConfig{},
			}

			err := tt.sut.ApplyResources(t.Context(), fakeClient, agentApplyOptions)
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
