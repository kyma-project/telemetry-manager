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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	namespace = "my-namespace"
	name      = "my-self-monitor"
	cfg       = "dummy self-monitor config"
)

var (
	baseCPURequest    = resource.MustParse("10m")
	baseCPULimit      = resource.MustParse("30m")
	baseMemoryRequest = resource.MustParse("15Mi")
	baseMemoryLimit   = resource.MustParse("30Mi")
)

func TestRemoveSelfMonitorResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	selfMonConfig := makeSelfMonitorConfig()
	err := ApplyResources(ctx, client, selfMonConfig)
	require.NoError(t, err)

	t.Run("It should create all resources", func(t *testing.T) {
		verifyConfigMapIsPresent(ctx, t, client)
		verifyDeploymentIsPreset(ctx, t, client)
		verifyRoleIsPresent(ctx, t, client)
		verifyRoleBindingIsPresent(ctx, t, client)
		verifyServiceAccountIsPresent(ctx, t, client)
		verifyNetworkPolicy(ctx, t, client)
	})

	err = RemoveResources(ctx, client, selfMonConfig)
	require.NoError(t, err)

	t.Run("Deployment should not be present", func(t *testing.T) {
		var deps appsv1.DeploymentList
		require.NoError(t, client.List(ctx, &deps))
		require.Len(t, deps.Items, 0)
	})

	t.Run("Configmap should not be present", func(t *testing.T) {
		var cms corev1.ConfigMapList
		require.NoError(t, client.List(ctx, &cms))
		require.Len(t, cms.Items, 0)
	})
	t.Run("role should not be present", func(t *testing.T) {
		var roles rbacv1.RoleList
		require.NoError(t, client.List(ctx, &roles))
		require.Len(t, roles.Items, 0)
	})
	t.Run("role binding should not be present", func(t *testing.T) {
		var roleBindings rbacv1.RoleBindingList
		require.NoError(t, client.List(ctx, &roleBindings))
		require.Len(t, roleBindings.Items, 0)
	})
	t.Run("network policy should not be present", func(t *testing.T) {
		var nwPs networkingv1.NetworkPolicyList
		require.NoError(t, client.List(ctx, &nwPs))
		require.Len(t, nwPs.Items, 0)
	})

}

func TestApplySelfMonitorResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	err := ApplyResources(ctx, client, makeSelfMonitorConfig())
	require.NoError(t, err)

	t.Run("should create collector config configmap", func(t *testing.T) {
		verifyConfigMapIsPresent(ctx, t, client)
	})

	t.Run("should create a deployment", func(t *testing.T) {
		verifyDeploymentIsPreset(ctx, t, client)
	})

	t.Run("should create role", func(t *testing.T) {
		verifyRoleIsPresent(ctx, t, client)
	})

	t.Run("should create role binding", func(t *testing.T) {
		verifyRoleBindingIsPresent(ctx, t, client)
	})

	t.Run("should create service account", func(t *testing.T) {
		verifyServiceAccountIsPresent(ctx, t, client)
	})

	t.Run("should create network policy", func(t *testing.T) {
		verifyNetworkPolicy(ctx, t, client)
	})

}

func verifyDeploymentIsPreset(ctx context.Context, t *testing.T, client client.Client) {
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
}

func verifyConfigMapIsPresent(ctx context.Context, t *testing.T, client client.Client) {
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
}

func verifyRoleIsPresent(ctx context.Context, t *testing.T, client client.Client) {
	var rs rbacv1.RoleList
	require.NoError(t, client.List(ctx, &rs))
	require.Len(t, rs.Items, 1)

	r := rs.Items[0]
	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	require.NotNil(t, r)
	require.Equal(t, r.Name, name)
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name": name,
	}, r.Labels)
	require.Equal(t, r.Rules, expectedRules)
}

func verifyRoleBindingIsPresent(ctx context.Context, t *testing.T, client client.Client) {
	var rbs rbacv1.RoleBindingList
	require.NoError(t, client.List(ctx, &rbs))
	require.Len(t, rbs.Items, 1)

	rb := rbs.Items[0]
	require.NotNil(t, rb)
	require.Equal(t, name, rb.Name)
	require.Equal(t, namespace, rb.Namespace)
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name": name,
	}, rb.Labels)
	require.Equal(t, name, rb.RoleRef.Name)
}

func verifyServiceAccountIsPresent(ctx context.Context, t *testing.T, client client.Client) {
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
}

func verifyNetworkPolicy(ctx context.Context, t *testing.T, client client.Client) {
	expectedTelemetryPodSelector := map[string]string{
		"self-monitor/access": "true",
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
	require.Equal(t, expectedNamespaceSelector, np.Spec.Egress[0].To[0].NamespaceSelector.MatchLabels)
}

func makeSelfMonitorConfig() *Config {
	return &Config{
		BaseName:          name,
		Namespace:         namespace,
		SelfMonitorConfig: cfg,
		Deployment: DeploymentConfig{
			Image:         "foo.bar",
			CPULimit:      baseCPULimit,
			CPURequest:    baseCPURequest,
			MemoryLimit:   baseMemoryLimit,
			MemoryRequest: baseMemoryRequest,
		},
	}
}
