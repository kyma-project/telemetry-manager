package fluentbit

import (
	"context"
	"os"
	"regexp"
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

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestAgent_ApplyResources(t *testing.T) {
	globals := config.NewGlobal(
		config.WithTargetNamespace("kyma-system"),
		config.WithImagePullSecretName("mySecret"),
		config.WithAdditionalWorkloadLabels(map[string]string{"test-label-key": "test-label-value"}),
		config.WithAdditionalWorkloadAnnotations(map[string]string{"test-anno-key": "test-anno-value"}),
		config.WithClusterTrustBundleName("trustBundle"),
	)
	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	initContainerImage := "alpine"
	priorityClassName := "foo-prio-class"
	namespace := "kyma-system"

	tests := []struct {
		name           string
		sut            *AgentApplierDeleter
		goldenFilePath string
	}{
		{
			name:           "fluentbit",
			sut:            NewFluentBitApplierDeleter(globals, namespace, image, exporterImage, initContainerImage, priorityClassName),
			goldenFilePath: "testdata/fluentbit.yaml",
		},
	}

	for _, tt := range tests {
		var objects []client.Object

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
		utilruntime.Must(telemetryv1beta1.AddToScheme(scheme))

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
				objects = append(objects, obj)
				// Nothing has to be created, just add created object to the list
				return nil
			},
		}).Build()

		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(t.Context(), fakeClient, AgentApplyOptions{
				IstioEnabled: false,
				FluentBitConfig: &builder.FluentBitConfig{
					SectionsConfig:  map[string]string{"pipeline1.conf": "dummy-sections-content"},
					FilesConfig:     map[string]string{"file1": "dummy-file-content"},
					EnvConfigSecret: map[string][]byte{"env-config-secret1": []byte("dummy-value")},
					TLSConfigSecret: map[string][]byte{"tls-config-secret1": []byte("dummy-value")},
				},
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
			sut:  NewFluentBitApplierDeleter(globals, namespace, image, exporterImage, initContainerImage, priorityClassName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentApplyOptions := AgentApplyOptions{
				IstioEnabled:    false,
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

func TestK8sPodsParserRegex(t *testing.T) {
	// This regex is used in the k8s-pods parser (line 575 in resources.go)
	// It parses Kubernetes pod log file names to extract namespace, pod name, and container name
	k8sPodsRegex := regexp.MustCompile(`^(?<namespace_name>[^_]+)_(?<pod_name>[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?:\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)_[a-f0-9\-]{36}\.(?<container_name>[^\.]+)\.\d+\.log$`)

	tests := []struct {
		name            string
		logFileName     string
		shouldMatch     bool
		expectedCapture map[string]string
	}{
		{
			name:        "standard pod log file",
			logFileName: "default_nginx-deployment-7d64f8b8b4-5xq9z_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.nginx.0.log",
			shouldMatch: true,
			expectedCapture: map[string]string{
				"namespace_name": "default",
				"pod_name":       "nginx-deployment-7d64f8b8b4-5xq9z",
				"container_name": "nginx",
			},
		},
		{
			name:        "pod with dots in name",
			logFileName: "kube-system_coredns-1234567890-abcde.v2_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.coredns.0.log",
			shouldMatch: true,
			expectedCapture: map[string]string{
				"namespace_name": "kube-system",
				"pod_name":       "coredns-1234567890-abcde.v2",
				"container_name": "coredns",
			},
		},
		{
			name:        "namespace with hyphens",
			logFileName: "kyma-system_telemetry-manager-5f6g7h8i9j-klmno_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.manager.1.log",
			shouldMatch: true,
			expectedCapture: map[string]string{
				"namespace_name": "kyma-system",
				"pod_name":       "telemetry-manager-5f6g7h8i9j-klmno",
				"container_name": "manager",
			},
		},
		{
			name:        "container with hyphens",
			logFileName: "default_app-pod-123_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.init-container.0.log",
			shouldMatch: true,
			expectedCapture: map[string]string{
				"namespace_name": "default",
				"pod_name":       "app-pod-123",
				"container_name": "init-container",
			},
		},
		{
			name:        "log file with higher rotation number",
			logFileName: "production_backend-service-abc123_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.backend.99.log",
			shouldMatch: true,
			expectedCapture: map[string]string{
				"namespace_name": "production",
				"pod_name":       "backend-service-abc123",
				"container_name": "backend",
			},
		},
		{
			name:        "single character pod name",
			logFileName: "default_a_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.app.0.log",
			shouldMatch: true,
			expectedCapture: map[string]string{
				"namespace_name": "default",
				"pod_name":       "a",
				"container_name": "app",
			},
		},
		{
			name:        "invalid: pod name starts with hyphen",
			logFileName: "default_-invalid-pod_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.app.0.log",
			shouldMatch: false,
		},
		{
			name:        "invalid: pod name ends with hyphen",
			logFileName: "default_invalid-pod-_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.app.0.log",
			shouldMatch: false,
		},
		{
			name:        "invalid: pod name contains uppercase",
			logFileName: "default_Invalid-Pod_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.app.0.log",
			shouldMatch: false,
		},
		{
			name:        "invalid: missing UUID",
			logFileName: "default_nginx-pod.nginx.0.log",
			shouldMatch: false,
		},
		{
			name:        "invalid: short UUID",
			logFileName: "default_nginx-pod_8e9f6a0b.nginx.0.log",
			shouldMatch: false,
		},
		{
			name:        "invalid: missing container name",
			logFileName: "default_nginx-pod_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.0.log",
			shouldMatch: false,
		},
		{
			name:        "invalid: missing log extension",
			logFileName: "default_nginx-pod_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.nginx.0",
			shouldMatch: false,
		},
		{
			name:        "invalid: non-numeric rotation number",
			logFileName: "default_nginx-pod_8e9f6a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b.nginx.abc.log",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := k8sPodsRegex.FindStringSubmatch(tt.logFileName)

			if tt.shouldMatch {
				require.NotNil(t, matches, "expected regex to match log file name: %s", tt.logFileName)
				require.Greater(t, len(matches), 0, "expected at least one match")

				// Extract named groups
				for name, expectedValue := range tt.expectedCapture {
					idx := k8sPodsRegex.SubexpIndex(name)
					require.Greater(t, idx, 0, "named group %s not found in regex", name)
					require.Equal(t, expectedValue, matches[idx], "unexpected value for named group %s", name)
				}
			} else {
				require.Nil(t, matches, "expected regex to NOT match log file name: %s", tt.logFileName)
			}
		})
	}
}
