package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyAgentResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()
	namespace := "my-namespace"
	name := "my-application"

	agentConfig := &AgentConfig{
		Config: Config{
			BaseName:  name,
			Namespace: namespace,
		},
	}

	err := ApplyAgentResources(ctx, client, agentConfig)
	require.NoError(t, err)

	t.Run("should create a daemonset", func(t *testing.T) {
		var daemonSets appsv1.DaemonSetList
		require.NoError(t, client.List(ctx, &daemonSets))
		require.Len(t, daemonSets.Items, 1)

		ds := daemonSets.Items[0]
		require.Equal(t, name, ds.Name)
		require.Equal(t, namespace, ds.Namespace)

		//labels
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, ds.Labels, "must have expected daemonset labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, ds.Spec.Selector.MatchLabels, "must have expected daemonset selector labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":  name,
			"sidecar.istio.io/inject": "true",
		}, ds.Spec.Template.ObjectMeta.Labels, "must have expected pod labels")

		//annotations
		podAnnotations := ds.Spec.Template.ObjectMeta.Annotations
		require.NotEmpty(t, podAnnotations["checksum/config"])
		require.Equal(t, "# configure an env variable OUTPUT_CERTS to write certificates to the given folder\nproxyMetadata:\n  OUTPUT_CERTS: /etc/istio-output-certs\n", podAnnotations["proxy.istio.io/config"])
		require.Equal(t, "[{\"name\": \"istio-certs\", \"mountPath\": \"/etc/istio-output-certs\"}]", podAnnotations["sidecar.istio.io/userVolumeMount"])
		require.Equal(t, "", podAnnotations["traffic.sidecar.istio.io/includeInboundPorts"])
		require.Equal(t, "4317", podAnnotations["traffic.sidecar.istio.io/includeOutboundPorts"])

		//collector container
		require.Len(t, ds.Spec.Template.Spec.Containers, 1)
		container := ds.Spec.Template.Spec.Containers[0]

		require.NotNil(t, container.LivenessProbe, "liveness probe must be defined")
		require.NotNil(t, container.ReadinessProbe, "readiness probe must be defined")

		envVars := container.Env
		require.Len(t, envVars, 2)
		require.Equal(t, envVars[0].Name, "MY_POD_IP")
		require.Equal(t, envVars[1].Name, "MY_NODE_NAME")
		require.Equal(t, envVars[0].ValueFrom.FieldRef.FieldPath, "status.podIP")
		require.Equal(t, envVars[1].ValueFrom.FieldRef.FieldPath, "spec.nodeName")

		//security contexts
		podSecurityContext := ds.Spec.Template.Spec.SecurityContext
		require.NotNil(t, podSecurityContext, "pod security context must be defined")
		require.NotZero(t, podSecurityContext.RunAsUser, "must run as non-root")
		require.True(t, *podSecurityContext.RunAsNonRoot, "must run as non-root")

		containerSecurityContext := container.SecurityContext
		require.NotNil(t, containerSecurityContext, "container security context must be defined")
		require.NotZero(t, containerSecurityContext.RunAsUser, "must run as non-root")
		require.True(t, *containerSecurityContext.RunAsNonRoot, "must run as non-root")
		require.False(t, *containerSecurityContext.Privileged, "must not be privileged")
		require.False(t, *containerSecurityContext.AllowPrivilegeEscalation, "must not escalate to privileged")
		require.True(t, *containerSecurityContext.ReadOnlyRootFilesystem, "must use readonly fs")
	})
}

//var (
//	config = Config{
//		BaseName:  "collector",
//		Namespace: "telemetry-system",
//	}
//)
//
//func TestMakeClusterRole(t *testing.T) {
//	name := types.NamespacedName{Name: "telemetry-metric-agent", Namespace: "telemetry-system"}
//	clusterRole := MakeClusterRole(name)
//	expectedRules := []rbacv1.PolicyRule{
//		{
//			APIGroups: []string{""},
//			Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
//			Verbs:     []string{"get", "list", "watch"},
//		},
//		{
//			NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
//			Verbs:           []string{"get"},
//		},
//	}
//
//	require.NotNil(t, clusterRole)
//	require.Equal(t, clusterRole.Name, name.Name)
//	require.Equal(t, clusterRole.Rules, expectedRules)
//}
//
//func TestMakeDaemonSet(t *testing.T) {
//	daemonSet := MakeDaemonSet(config, "123", "MY_POD_IP", "MY_NODE_NAME", "/etc/istio/certs")
//

//}
