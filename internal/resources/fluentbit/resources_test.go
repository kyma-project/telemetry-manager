package fluentbit

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"

	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestAgent_ApplyResources(t *testing.T) {
	image := "foo-fluentbit"
	exporterImage := "foo-exporter"
	priorityClassName := "foo-prio-class"
	logPipeline := testutils.NewLogPipelineBuilder().WithName("foo-logpipeline").Build()

	tests := []struct {
		name           string
		sut            *AgentApplierDeleter
		goldenFilePath string
		saveGoldenFile bool
	}{
		{
			name:           "fluentbit",
			sut:            NewFluentBitApplierDeleter(image, exporterImage, priorityClassName),
			goldenFilePath: "testdata/fluentbit.yaml",
		},
	}

	for _, tt := range tests {
		var objects []client.Object

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
		utilruntime.Must(v1alpha1.AddToScheme(scheme))

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			Create: func(_ context.Context, c client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
				objects = append(objects, obj)
				// Nothing has to be created, just add created object to the list
				return nil
			},
			// Update interceptor is needed for syncSectionsConfigMap operation
			Update: func(_ context.Context, c client.WithWatch, obj client.Object, option ...client.UpdateOption) error {
				// For updates, we'll either update the existing object in our slice
				// or append it if it doesn't exist
				found := false
				for i, existingObj := range objects {
					if existingObj.GetName() == obj.GetName() && existingObj.GetNamespace() == obj.GetNamespace() {
						objects[i] = obj
						found = true
						break
					}
				}
				if !found {
					objects = append(objects, obj)
				}
				return nil
			},
			Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				// Simulate that the object exists but is empty
				// This is needed for GetOrCreate operations
				return nil
			},
		}).Build()

		t.Run(tt.name, func(t *testing.T) {
			err := tt.sut.ApplyResources(context.Background(), fakeClient, AgentApplyOptions{
				Config: Config{
					DaemonSet:           types.NamespacedName{Name: "foo-daemonset", Namespace: "kyma-system"},
					SectionsConfigMap:   types.NamespacedName{Name: "foo-sectionscm", Namespace: "kyma-system"},
					FilesConfigMap:      types.NamespacedName{Name: "foo-filescm", Namespace: "kyma-system"},
					LuaConfigMap:        types.NamespacedName{Name: "foo-luacm", Namespace: "kyma-system"},
					ParsersConfigMap:    types.NamespacedName{Name: "foo-parserscm", Namespace: "kyma-system"},
					EnvConfigSecret:     types.NamespacedName{Name: "foo-evnconfigsecret", Namespace: "kyma-system"},
					TLSFileConfigSecret: types.NamespacedName{Name: "foo-tlsfileconfigsecret", Namespace: "kyma-system"},
				},
				AllowedPorts: []int32{5555, 6666},

				Pipeline:               &logPipeline,
				DeployableLogPipelines: []v1alpha1.LogPipeline{logPipeline},
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
	priorityClassName := "foo-prio-class"
	logPipeline := testutils.NewLogPipelineBuilder().WithName("foo-logpipeline").Build()

	var created []client.Object

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

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
			sut:  NewFluentBitApplierDeleter(image, exporterImage, priorityClassName),
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			agentApplyOptions := AgentApplyOptions{
				Config: Config{
					DaemonSet:           types.NamespacedName{Name: "foo-daemonset", Namespace: "kyma-system"},
					SectionsConfigMap:   types.NamespacedName{Name: "foo-sectionscm", Namespace: "kyma-system"},
					FilesConfigMap:      types.NamespacedName{Name: "foo-filescm", Namespace: "kyma-system"},
					LuaConfigMap:        types.NamespacedName{Name: "foo-luacm", Namespace: "kyma-system"},
					ParsersConfigMap:    types.NamespacedName{Name: "foo-parserscm", Namespace: "kyma-system"},
					EnvConfigSecret:     types.NamespacedName{Name: "foo-evnconfigsecret", Namespace: "kyma-system"},
					TLSFileConfigSecret: types.NamespacedName{Name: "foo-tlsfileconfigsecret", Namespace: "kyma-system"},
				},
				AllowedPorts:           []int32{5555, 6666},
				Pipeline:               &logPipeline,
				DeployableLogPipelines: []v1alpha1.LogPipeline{logPipeline},
			}

			err := tt.sut.ApplyResources(context.Background(), fakeClient, agentApplyOptions)
			require.NoError(t, err)

			err = tt.sut.DeleteResources(context.Background(), fakeClient, agentApplyOptions)
			require.NoError(t, err)

			for i := range created {
				// an update operation on a non-existent object should return a NotFound error
				err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(created[i]), created[i])
				require.True(t, apierrors.IsNotFound(err), "want not found, got %v: %#v", err, created[i])
			}
		})
	}
}

//func TestMakeDaemonSet(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
//	checksum := "foo"
//	ds := DaemonSetConfig{
//		FluentBitImage:              "foo-fluenbit",
//		FluentBitConfigPrepperImage: "foo-configprepper",
//		ExporterImage:               "foo-exporter",
//		PriorityClassName:           "foo-prio-class",
//		MemoryLimit:                 resource.MustParse("400Mi"),
//		CPURequest:                  resource.MustParse(".1"),
//		MemoryRequest:               resource.MustParse("100Mi"),
//	}
//
//	expectedAnnotations := map[string]string{
//		"checksum/logpipeline-config":                  checksum,
//		"traffic.sidecar.istio.io/excludeInboundPorts": "2020,2021",
//	}
//	daemonSet := MakeDaemonSet(name, checksum, ds)
//
//	require.NotNil(t, daemonSet)
//	require.Equal(t, daemonSet.Name, name.Name)
//	require.Equal(t, daemonSet.Namespace, name.Namespace)
//	require.Equal(t, map[string]string{
//		"app.kubernetes.io/name":     "fluent-bit",
//		"app.kubernetes.io/instance": "telemetry",
//	}, daemonSet.Spec.Selector.MatchLabels)
//	require.Equal(t, map[string]string{
//		"app.kubernetes.io/name":               "fluent-bit",
//		"app.kubernetes.io/instance":           "telemetry",
//		"sidecar.istio.io/inject":              "true",
//		"telemetry.kyma-project.io/log-export": "true",
//	}, daemonSet.Spec.Template.ObjectMeta.Labels)
//	require.NotEmpty(t, daemonSet.Spec.Template.Spec.Containers[0].EnvFrom)
//	require.NotNil(t, daemonSet.Spec.Template.Spec.Containers[0].LivenessProbe, "liveness probe must be defined")
//	require.NotNil(t, daemonSet.Spec.Template.Spec.Containers[0].ReadinessProbe, "readiness probe must be defined")
//	require.Equal(t, daemonSet.Spec.Template.ObjectMeta.Annotations, expectedAnnotations, "annotations should contain istio port exclusion of 2020 and 2021")
//	podSecurityContext := daemonSet.Spec.Template.Spec.SecurityContext
//	require.NotNil(t, podSecurityContext, "pod security context must be defined")
//	require.False(t, *podSecurityContext.RunAsNonRoot, "must not run as non-root")
//
//	resources := daemonSet.Spec.Template.Spec.Containers[0].Resources
//	require.Equal(t, ds.CPURequest, *resources.Requests.Cpu(), "cpu requests should be defined")
//	require.Equal(t, ds.MemoryRequest, *resources.Requests.Memory(), "memory requests should be defined")
//	require.Equal(t, ds.MemoryLimit, *resources.Limits.Memory(), "memory limit should be defined")
//
//	containerSecurityContext := daemonSet.Spec.Template.Spec.Containers[0].SecurityContext
//	require.NotNil(t, containerSecurityContext, "container security context must be defined")
//	require.False(t, *containerSecurityContext.Privileged, "must not be privileged")
//	require.False(t, *containerSecurityContext.AllowPrivilegeEscalation, "must not escalate to privileged")
//	require.True(t, *containerSecurityContext.ReadOnlyRootFilesystem, "must use readonly fs")
//
//	volMounts := daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts
//	require.Equal(t, 10, len(volMounts), "volume mounts do not match")
//}
//
//func TestMakeClusterRole(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
//	clusterRole := MakeClusterRole(name)
//	expectedRules := []rbacv1.PolicyRule{
//		{
//			APIGroups: []string{""},
//			Resources: []string{"namespaces", "pods"},
//			Verbs:     []string{"get", "list", "watch"},
//		},
//	}
//
//	require.NotNil(t, clusterRole)
//	require.Equal(t, clusterRole.Name, name.Name)
//	require.Equal(t, clusterRole.Rules, expectedRules)
//}
//
//func TestMakeMetricsService(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
//	service := MakeMetricsService(name)
//
//	require.NotNil(t, service)
//	require.Equal(t, service.Name, "telemetry-fluent-bit-metrics")
//	require.Equal(t, service.Namespace, name.Namespace)
//	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
//	require.Len(t, service.Spec.Ports, 1)
//
//	require.Contains(t, service.Labels, "telemetry.kyma-project.io/self-monitor")
//
//	require.Contains(t, service.Annotations, "prometheus.io/scrape")
//	require.Contains(t, service.Annotations, "prometheus.io/port")
//	require.Contains(t, service.Annotations, "prometheus.io/scheme")
//	require.Contains(t, service.Annotations, "prometheus.io/path")
//
//	port, err := strconv.ParseInt(service.Annotations["prometheus.io/port"], 10, 16)
//	require.NoError(t, err)
//	require.Equal(t, int32(port), service.Spec.Ports[0].Port) //nolint:gosec // parseInt returns int64.  This is a testfile so not part of binary
//}
//
//func TestMakeExporterMetricsService(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
//	service := MakeExporterMetricsService(name)
//
//	require.NotNil(t, service)
//	require.Equal(t, service.Name, "telemetry-fluent-bit-exporter-metrics")
//	require.Equal(t, service.Namespace, name.Namespace)
//	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
//	require.Len(t, service.Spec.Ports, 1)
//
//	require.Contains(t, service.Labels, "telemetry.kyma-project.io/self-monitor")
//
//	require.Contains(t, service.Annotations, "prometheus.io/scrape")
//	require.Contains(t, service.Annotations, "prometheus.io/port")
//	require.Contains(t, service.Annotations, "prometheus.io/scheme")
//
//	port, err := strconv.ParseInt(service.Annotations["prometheus.io/port"], 10, 16)
//	require.NoError(t, err)
//	require.Equal(t, int32(port), service.Spec.Ports[0].Port) //nolint:gosec // parseInt returns int64.  This is a testfile so not part of binary
//}
//
//func TestMakeConfigMap(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
//	cm := MakeConfigMap(name)
//
//	require.NotNil(t, cm)
//	require.Equal(t, cm.Name, name.Name)
//	require.Equal(t, cm.Namespace, name.Namespace)
//	require.NotEmpty(t, cm.Data["custom_parsers.conf"])
//	require.NotEmpty(t, cm.Data["fluent-bit.conf"])
//}
//
//func TestMakeLuaConfigMap(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-fluent-bit-luascripts", Namespace: "telemetry-system"}
//	cm := MakeLuaConfigMap(name)
//
//	require.NotNil(t, cm)
//	require.Equal(t, cm.Name, name.Name)
//	require.Equal(t, cm.Namespace, name.Namespace)
//	require.NotEmpty(t, cm.Data["filter-script.lua"])
//}
