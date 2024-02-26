package selfmonitor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplySelfMonitorResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()
	namespace := "my-namespace"
	name := "my-self-monitor"
	cfg := "dummy self-monitor config"

	baseCPURequest := resource.MustParse("10m")
	baseCPULimit := resource.MustParse("30m")
	baseMemoryRequest := resource.MustParse("15Mi")
	baseMemoryLimit := resource.MustParse("30Mi")

	selfMonConfig := &Config{
		BaseName:         name,
		Namespace:        namespace,
		monitoringConfig: cfg,
		Deployment: DeploymentConfig{
			Image:         "foo.bar",
			CPULimit:      baseCPULimit,
			CPURequest:    baseCPURequest,
			MemoryLimit:   baseMemoryLimit,
			MemoryRequest: baseMemoryRequest,
		},
	}

	err := ApplyResources(ctx, client, selfMonConfig)
	require.NoError(t, err)

	t.Run("should create collector config configmap", func(t *testing.T) {
		var cms corev1.ConfigMapList
		require.NoError(t, client.List(ctx, &cms))
		require.Len(t, cms.Items, 1)

		cm := cms.Items[0]
		require.Equal(t, name, cm.Name)
		require.Equal(t, namespace, cm.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, cm.Labels)
		require.Equal(t, cfg, cm.Data["prometheus.yml"])
	})

	t.Run("should create a deployment", func(t *testing.T) {
		var deps appsv1.DeploymentList
		require.NoError(t, client.List(ctx, &deps))
		require.Len(t, deps.Items, 1)

		dep := deps.Items[0]
		require.Equal(t, name, dep.Name)
		require.Equal(t, namespace, dep.Namespace)

		//labels
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, dep.Labels, "must have expected deployment labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, dep.Spec.Selector.MatchLabels, "must have expected deployment selector labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":  name,
			"sidecar.istio.io/inject": "false",
		}, dep.Spec.Template.ObjectMeta.Labels, "must have expected pod labels")

		//annotations
		podAnnotations := dep.Spec.Template.ObjectMeta.Annotations
		require.NotEmpty(t, podAnnotations["checksum/config"])

		//self-monitor container
		require.Len(t, dep.Spec.Template.Spec.Containers, 1)
		container := dep.Spec.Template.Spec.Containers[0]

		require.NotNil(t, container.LivenessProbe, "liveness probe must be defined")
		require.NotNil(t, container.ReadinessProbe, "readiness probe must be defined")
		resources := container.Resources
		require.Equal(t, baseCPURequest, *resources.Requests.Cpu(), "cpu requests should be defined")
		require.Equal(t, baseMemoryRequest, *resources.Requests.Memory(), "memory requests should be defined")
		require.Equal(t, baseCPULimit, *resources.Limits.Cpu(), "cpu limit should be defined")
		require.Equal(t, baseMemoryLimit, *resources.Limits.Memory(), "memory limit should be defined")

		//security contexts
		podSecurityContext := dep.Spec.Template.Spec.SecurityContext
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

	t.Run("should create clusterrole", func(t *testing.T) {
		var crs rbacv1.ClusterRoleList
		require.NoError(t, client.List(ctx, &crs))
		require.Len(t, crs.Items, 1)

		cr := crs.Items[0]
		expectedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}

		require.NotNil(t, cr)
		require.Equal(t, cr.Name, name)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, cr.Labels)
		require.Equal(t, cr.Rules, expectedRules)
	})

	t.Run("should create clusterrolebinding", func(t *testing.T) {
		var crbs rbacv1.ClusterRoleBindingList
		require.NoError(t, client.List(ctx, &crbs))
		require.Len(t, crbs.Items, 1)

		crb := crbs.Items[0]
		require.NotNil(t, crb)
		require.Equal(t, name, crb.Name)
		require.Equal(t, namespace, crb.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, crb.Labels)
		require.Equal(t, name, crb.RoleRef.Name)
	})

	t.Run("should create serviceaccount", func(t *testing.T) {
		var sas corev1.ServiceAccountList
		require.NoError(t, client.List(ctx, &sas))
		require.Len(t, sas.Items, 1)

		sa := sas.Items[0]
		require.NotNil(t, sa)
		require.Equal(t, name, sa.Name)
		require.Equal(t, namespace, sa.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, sa.Labels)
	})

	t.Run("should create networkpolicy", func(t *testing.T) {
		expectedTelemetryPodSelector := map[string]string{
			"app.kubernetes.io/instance": "telemetry",
			"control-plane":              "telemetry-operator",
		}
		expectedNamespaceSelector := map[string]string{
			"kubernetes.io/metadata.name": namespace,
		}

		var nps networkingv1.NetworkPolicyList
		require.NoError(t, client.List(ctx, &nps))
		require.Len(t, nps.Items, 1)

		np := nps.Items[0]
		require.NotNil(t, np)
		require.Equal(t, name, np.Name)
		require.Equal(t, namespace, np.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, np.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, np.Spec.PodSelector.MatchLabels)
		require.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress}, np.Spec.PolicyTypes)
		require.Len(t, np.Spec.Ingress, 1)
		require.Len(t, np.Spec.Ingress[0].From, 1)
		require.Equal(t, expectedNamespaceSelector, np.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels)
		require.Equal(t, expectedTelemetryPodSelector, np.Spec.Ingress[0].From[0].PodSelector.MatchLabels)
		require.Len(t, np.Spec.Ingress[0].Ports, 1)
		tcpProtocol := corev1.ProtocolTCP
		port9090 := intstr.FromInt32(9090)
		require.Equal(t, []networkingv1.NetworkPolicyPort{
			{
				Protocol: &tcpProtocol,
				Port:     &port9090,
			},
		}, np.Spec.Ingress[0].Ports)
		require.Len(t, np.Spec.Egress, 1)
		require.Len(t, np.Spec.Egress[0].To, 1)
		require.Equal(t, expectedNamespaceSelector, np.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels)
		require.Equal(t, expectedTelemetryPodSelector, np.Spec.Ingress[0].From[0].PodSelector.MatchLabels)

	})

}
