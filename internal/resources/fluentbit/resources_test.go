package fluentbit

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func TestMakeDaemonSet(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	checksum := "foo"
	ds := DaemonSetConfig{
		FluentBitImage:              "foo-fluenbit",
		FluentBitConfigPrepperImage: "foo-configprepper",
		ExporterImage:               "foo-exporter",
		PriorityClassName:           "foo-prio-class",
		CPULimit:                    resource.MustParse(".25"),
		MemoryLimit:                 resource.MustParse("400Mi"),
		CPURequest:                  resource.MustParse(".1"),
		MemoryRequest:               resource.MustParse("100Mi"),
	}

	expectedAnnotations := map[string]string{
		"checksum/logpipeline-config":                  checksum,
		"traffic.sidecar.istio.io/excludeInboundPorts": "2020,2021",
	}
	daemonSet := MakeDaemonSet(name, checksum, ds)

	require.NotNil(t, daemonSet)
	require.Equal(t, daemonSet.Name, name.Name)
	require.Equal(t, daemonSet.Namespace, name.Namespace)
	require.Equal(t, daemonSet.Spec.Selector.MatchLabels, Labels())
	require.Equal(t, daemonSet.Spec.Template.ObjectMeta.Labels, Labels())
	require.NotEmpty(t, daemonSet.Spec.Template.Spec.Containers[0].EnvFrom)
	require.NotNil(t, daemonSet.Spec.Template.Spec.Containers[0].LivenessProbe, "liveness probe must be defined")
	require.NotNil(t, daemonSet.Spec.Template.Spec.Containers[0].ReadinessProbe, "readiness probe must be defined")
	require.Equal(t, daemonSet.Spec.Template.ObjectMeta.Annotations, expectedAnnotations, "annotations should contain istio port exclusion of 2020 and 2021")
	podSecurityContext := daemonSet.Spec.Template.Spec.SecurityContext
	require.NotNil(t, podSecurityContext, "pod security context must be defined")
	require.False(t, *podSecurityContext.RunAsNonRoot, "must not run as non-root")

	resources := daemonSet.Spec.Template.Spec.Containers[0].Resources
	require.Equal(t, ds.CPURequest, *resources.Requests.Cpu(), "cpu requests should be defined")
	require.Equal(t, ds.MemoryRequest, *resources.Requests.Memory(), "memory requests should be defined")
	require.Equal(t, ds.CPULimit, *resources.Limits.Cpu(), "cpu limit should be defined")
	require.Equal(t, ds.MemoryLimit, *resources.Limits.Memory(), "memory limit should be defined")

	containerSecurityContext := daemonSet.Spec.Template.Spec.Containers[0].SecurityContext
	require.NotNil(t, containerSecurityContext, "container security context must be defined")
	require.False(t, *containerSecurityContext.Privileged, "must not be privileged")
	require.False(t, *containerSecurityContext.AllowPrivilegeEscalation, "must not escalate to privileged")
	require.True(t, *containerSecurityContext.ReadOnlyRootFilesystem, "must use readonly fs")

	volMounts := daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts
	require.Equal(t, 10, len(volMounts), "volume mounts do not match")
}

func TestMakeClusterRole(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	clusterRole := MakeClusterRole(name)
	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	require.NotNil(t, clusterRole)
	require.Equal(t, clusterRole.Name, name.Name)
	require.Equal(t, clusterRole.Rules, expectedRules)
}

func TestMakeMetricsService(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	service := MakeMetricsService(name)

	require.NotNil(t, service)
	require.Equal(t, service.Name, "telemetry-fluent-bit-metrics")
	require.Equal(t, service.Namespace, name.Namespace)
	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.Len(t, service.Spec.Ports, 1)

	require.Contains(t, service.Annotations, "prometheus.io/scrape")
	require.Contains(t, service.Annotations, "prometheus.io/port")
	require.Contains(t, service.Annotations, "prometheus.io/scheme")
	require.Contains(t, service.Annotations, "prometheus.io/path")

	port, err := strconv.ParseInt(service.Annotations["prometheus.io/port"], 10, 16)
	require.NoError(t, err)
	require.Equal(t, int32(port), service.Spec.Ports[0].Port)
}

func TestMakeExporterMetricsService(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	service := MakeExporterMetricsService(name)

	require.NotNil(t, service)
	require.Equal(t, service.Name, "telemetry-fluent-bit-exporter-metrics")
	require.Equal(t, service.Namespace, name.Namespace)
	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.Len(t, service.Spec.Ports, 1)

	require.Contains(t, service.Annotations, "prometheus.io/scrape")
	require.Contains(t, service.Annotations, "prometheus.io/port")
	require.Contains(t, service.Annotations, "prometheus.io/scheme")

	port, err := strconv.ParseInt(service.Annotations["prometheus.io/port"], 10, 16)
	require.NoError(t, err)
	require.Equal(t, int32(port), service.Spec.Ports[0].Port)
}

func TestMakeConfigMap(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	cm := MakeConfigMap(name, true)

	require.NotNil(t, cm)
	require.Equal(t, cm.Name, name.Name)
	require.Equal(t, cm.Namespace, name.Namespace)
	require.NotEmpty(t, cm.Data["custom_parsers.conf"])
	require.NotEmpty(t, cm.Data["fluent-bit.conf"])
}

func TestMakeLuaConfigMap(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit-luascripts", Namespace: "telemetry-system"}
	cm := MakeLuaConfigMap(name)

	require.NotNil(t, cm)
	require.Equal(t, cm.Name, name.Name)
	require.Equal(t, cm.Namespace, name.Namespace)
	require.NotEmpty(t, cm.Data["filter-script.lua"])
}
