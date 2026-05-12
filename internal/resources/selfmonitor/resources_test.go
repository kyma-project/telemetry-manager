package selfmonitor

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

const (
	namespace            = "my-namespace"
	prometheusConfigYAML = "dummy prometheus Config"
	alertRulesYAML       = "dummy alert rules"
	configPath           = "/dummy/"
	configFileName       = "dummy-config.yaml"
	alertRulesFileName   = "dummy-alerts.yaml"
)

func TestApplySelfMonitorResources(t *testing.T) {
	tests := []struct {
		name         string
		vpaEnabled   bool
		vpaCRDExists bool
		vpaMaxMemory string
		goldenFile   string
	}{
		{
			name:         "without VPA",
			vpaEnabled:   false,
			vpaCRDExists: false,
			goldenFile:   "testdata/self-monitor.yaml",
		},
		{
			name:         "with VPA enabled",
			vpaEnabled:   true,
			vpaCRDExists: true,
			vpaMaxMemory: "128Mi",
			goldenFile:   "testdata/self-monitor-with-vpa.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object

			ctx := t.Context()
			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

			client := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					objects = append(objects, obj)
					// Nothing has to be created, just add created object to the list
					return nil
				},
			}).Build()

			sut := ApplierDeleter{
				Config: Config{
					Global: config.NewGlobal(
						config.WithTargetNamespace(namespace),
						config.WithImagePullSecretName("mySecret"),
						config.WithAdditionalWorkloadLabels(map[string]string{"test-label-key": "test-label-value"}),
						config.WithAdditionalWorkloadAnnotations(map[string]string{"test-anno-key": "test-anno-value"}),
					),
				},
			}

			opts := ApplyOptions{
				AlertRulesFileName:       alertRulesFileName,
				AlertRulesYAML:           alertRulesYAML,
				PrometheusConfigFileName: configFileName,
				PrometheusConfigPath:     configPath,
				PrometheusConfigYAML:     prometheusConfigYAML,
				VpaCRDExists:             tt.vpaCRDExists,
				VpaEnabled:               tt.vpaEnabled,
			}

			if tt.vpaMaxMemory != "" {
				opts.VPAMaxAllowedMemory = resource.MustParse(tt.vpaMaxMemory)
			}

			err := sut.ApplyResources(ctx, client, opts)
			require.NoError(t, err)

			bytes, err := testutils.MarshalYAML(scheme, objects)
			require.NoError(t, err)

			if testutils.ShouldUpdateGoldenFiles() {
				testutils.UpdateGoldenFileYAML(t, tt.goldenFile, bytes)
				return
			}

			goldenFileBytes, err := os.ReadFile(tt.goldenFile)
			require.NoError(t, err)

			require.Equal(t, string(goldenFileBytes), string(bytes))
		})
	}
}

func TestDeleteSelfMonitorResources(t *testing.T) {
	tests := []struct {
		name         string
		vpaCRDExists bool
		vpaEnabled   bool
		vpaMaxMemory string
	}{
		{
			name:         "with VPA disabled",
			vpaCRDExists: false,
			vpaEnabled:   false,
		},
		{
			name:         "with VPA enabled",
			vpaCRDExists: true,
			vpaEnabled:   true,
			vpaMaxMemory: "128Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var created []client.Object

			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Create: func(ctx context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
						created = append(created, obj)
						return c.Create(ctx, obj)
					},
				}).Build()

			sut := ApplierDeleter{
				Config: Config{
					Global: config.NewGlobal(config.WithTargetNamespace(namespace)),
					Image:  "test-image:latest",
				},
			}

			opts := ApplyOptions{
				AlertRulesFileName:       alertRulesFileName,
				AlertRulesYAML:           alertRulesYAML,
				PrometheusConfigFileName: configFileName,
				PrometheusConfigPath:     configPath,
				PrometheusConfigYAML:     prometheusConfigYAML,
				VpaCRDExists:             tt.vpaCRDExists,
				VpaEnabled:               tt.vpaEnabled,
			}

			if tt.vpaMaxMemory != "" {
				opts.VPAMaxAllowedMemory = resource.MustParse(tt.vpaMaxMemory)
			}

			err := sut.ApplyResources(t.Context(), fakeClient, opts)
			require.NoError(t, err)

			// Verify VPA was created if enabled
			if tt.vpaCRDExists && tt.vpaEnabled {
				var vpa autoscalingvpav1.VerticalPodAutoscaler

				err = fakeClient.Get(t.Context(), types.NamespacedName{
					Name:      names.SelfMonitor,
					Namespace: namespace,
				}, &vpa)
				require.NoError(t, err, "VPA should exist after ApplyResources with VPA enabled")
			}

			err = sut.DeleteResources(t.Context(), fakeClient, tt.vpaCRDExists)
			require.NoError(t, err)

			// Verify all created resources are deleted
			for i := range created {
				err = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(created[i]), created[i])
				require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
			}

			// Specifically verify VPA is deleted if it was created
			if tt.vpaCRDExists {
				var vpa autoscalingvpav1.VerticalPodAutoscaler

				err = fakeClient.Get(t.Context(), types.NamespacedName{
					Name:      names.SelfMonitor,
					Namespace: namespace,
				}, &vpa)
				require.True(t, apierrors.IsNotFound(err), "VPA should be deleted after DeleteResources")
			}
		})
	}
}
