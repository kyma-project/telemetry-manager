package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	config = Config{
		BaseName:  "collector",
		Namespace: "telemetry-system",
	}
)

func TestMakeClusterRole(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-metric-agent", Namespace: "telemetry-system"}
	clusterRole := MakeClusterRole(name)
	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
			Verbs:           []string{"get"},
		},
	}

	require.NotNil(t, clusterRole)
	require.Equal(t, clusterRole.Name, name.Name)
	require.Equal(t, clusterRole.Rules, expectedRules)
}

func TestMakeDaemonSet(t *testing.T) {
	daemonSet := MakeDaemonSet(config, "123", "MY_POD_IP", "MY_NODE_NAME", "/etc/istio-output-certs")

	require.NotNil(t, daemonSet)
	require.Equal(t, daemonSet.Name, config.BaseName)
	require.Equal(t, daemonSet.Namespace, config.Namespace)

	expectedLabels := map[string]string{"app.kubernetes.io/name": config.BaseName}
	require.Equal(t, daemonSet.Spec.Selector.MatchLabels, expectedLabels)
	require.Equal(t, daemonSet.Spec.Template.ObjectMeta.Labels, expectedLabels)

	require.Equal(t, daemonSet.Spec.Template.ObjectMeta.Annotations["sidecar.istio.io/inject"], "false")
	require.Equal(t, daemonSet.Spec.Template.ObjectMeta.Annotations["checksum/config"], "123")
	require.NotEmpty(t, daemonSet.Spec.Template.Spec.Containers[0].EnvFrom)

	require.NotNil(t, daemonSet.Spec.Template.Spec.Containers[0].LivenessProbe, "liveness probe must be defined")
	require.NotNil(t, daemonSet.Spec.Template.Spec.Containers[0].ReadinessProbe, "readiness probe must be defined")

	podSecurityContext := daemonSet.Spec.Template.Spec.SecurityContext
	require.NotNil(t, podSecurityContext, "pod security context must be defined")
	require.NotZero(t, podSecurityContext.RunAsUser, "must run as non-root")
	require.True(t, *podSecurityContext.RunAsNonRoot, "must run as non-root")

	containerSecurityContext := daemonSet.Spec.Template.Spec.Containers[0].SecurityContext
	require.NotNil(t, containerSecurityContext, "container security context must be defined")
	require.NotZero(t, containerSecurityContext.RunAsUser, "must run as non-root")
	require.True(t, *containerSecurityContext.RunAsNonRoot, "must run as non-root")
	require.False(t, *containerSecurityContext.Privileged, "must not be privileged")
	require.False(t, *containerSecurityContext.AllowPrivilegeEscalation, "must not escalate to privileged")
	require.True(t, *containerSecurityContext.ReadOnlyRootFilesystem, "must use readonly fs")
}
