package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyAgentResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()
	namespace := "my-namespace"
	name := "my-agent"
	cfg := "dummy otel collector config"

	agentConfig := &AgentConfig{
		allowedPorts: []int32{5555, 6666},
		Config: Config{
			BaseName:        name,
			Namespace:       namespace,
			CollectorConfig: cfg,
		},
	}

	err := ApplyAgentResources(ctx, client, agentConfig)
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
		require.Contains(t, cm.Data, "relay.conf")
		require.Equal(t, cfg, cm.Data["relay.conf"])
	})

	t.Run("should create a daemonset", func(t *testing.T) {
		var dss appsv1.DaemonSetList
		require.NoError(t, client.List(ctx, &dss))
		require.Len(t, dss.Items, 1)

		ds := dss.Items[0]
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
		require.Equal(t, "8888", podAnnotations["traffic.sidecar.istio.io/excludeInboundPorts"])
		require.Equal(t, "", podAnnotations["traffic.sidecar.istio.io/includeOutboundIPRanges"])

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

	t.Run("should create clusterrole", func(t *testing.T) {
		var crs rbacv1.ClusterRoleList
		require.NoError(t, client.List(ctx, &crs))
		require.Len(t, crs.Items, 1)

		cr := crs.Items[0]
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
		require.Len(t, np.Spec.Ingress[0].From, 2)
		require.Equal(t, "0.0.0.0/0", np.Spec.Ingress[0].From[0].IPBlock.CIDR)
		require.Equal(t, "::/0", np.Spec.Ingress[0].From[1].IPBlock.CIDR)
		require.Len(t, np.Spec.Ingress[0].Ports, 2)
		tcpProtocol := corev1.ProtocolTCP
		port5555 := intstr.FromInt32(5555)
		port6666 := intstr.FromInt32(6666)
		require.Equal(t, []networkingv1.NetworkPolicyPort{
			{
				Protocol: &tcpProtocol,
				Port:     &port5555,
			},
			{
				Protocol: &tcpProtocol,
				Port:     &port6666,
			},
		}, np.Spec.Ingress[0].Ports)
		require.Len(t, np.Spec.Egress, 1)
		require.Len(t, np.Spec.Egress[0].To, 2)
		require.Equal(t, "0.0.0.0/0", np.Spec.Egress[0].To[0].IPBlock.CIDR)
		require.Equal(t, "::/0", np.Spec.Egress[0].To[1].IPBlock.CIDR)
	})

	t.Run("should create metrics service", func(t *testing.T) {
		var svcs corev1.ServiceList
		require.NoError(t, client.List(ctx, &svcs))
		require.Len(t, svcs.Items, 1)

		svc := svcs.Items[0]
		require.NotNil(t, svc)
		require.Equal(t, name+"-metrics", svc.Name)
		require.Equal(t, namespace, svc.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, svc.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, svc.Spec.Selector)
		require.Equal(t, map[string]string{
			"prometheus.io/port":   "8888",
			"prometheus.io/scheme": "http",
			"prometheus.io/scrape": "true",
		}, svc.Annotations)
		require.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
		require.Len(t, svc.Spec.Ports, 1)
		require.Equal(t, corev1.ServicePort{
			Name:       "http-metrics",
			Protocol:   corev1.ProtocolTCP,
			Port:       8888,
			TargetPort: intstr.FromInt32(8888),
		}, svc.Spec.Ports[0])
	})
}
