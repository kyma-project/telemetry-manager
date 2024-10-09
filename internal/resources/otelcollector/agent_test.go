package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	agentNamespace = "my-namespace"
	agentName      = "my-agent"
	agentCfg       = "dummy otel collector config"
)

func TestApplyAgentResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	sut := AgentApplierDeleter{
		Config: AgentConfig{
			Config: Config{
				BaseName:  agentName,
				Namespace: agentNamespace,
			},
		},
		RBAC: createAgentRBAC(),
	}

	err := sut.ApplyResources(ctx, client, AgentApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: agentCfg,
	})
	require.NoError(t, err)

	t.Run("should create service account", func(t *testing.T) {
		var sas corev1.ServiceAccountList
		require.NoError(t, client.List(ctx, &sas))
		require.Len(t, sas.Items, 1)

		sa := sas.Items[0]
		require.NotNil(t, sa)
		require.Equal(t, agentName, sa.Name)
		require.Equal(t, agentNamespace, sa.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, sa.Labels)
	})

	t.Run("should create cluster role", func(t *testing.T) {
		var crs rbacv1.ClusterRoleList
		require.NoError(t, client.List(ctx, &crs))
		require.Len(t, crs.Items, 1)

		cr := crs.Items[0]
		require.NotNil(t, cr)
		require.Equal(t, agentName, cr.Name)
		require.Equal(t, agentNamespace, cr.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, cr.Labels)
		require.Equal(t, sut.RBAC.clusterRole.Rules, cr.Rules)
	})

	t.Run("should create cluster role binding", func(t *testing.T) {
		var crbs rbacv1.ClusterRoleBindingList
		require.NoError(t, client.List(ctx, &crbs))
		require.Len(t, crbs.Items, 1)

		crb := crbs.Items[0]
		require.NotNil(t, crb)
		require.Equal(t, agentName, crb.Name)
		require.Equal(t, agentNamespace, crb.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, crb.Labels)

		subject := crb.Subjects[0]
		require.Equal(t, "ServiceAccount", subject.Kind)
		require.Equal(t, agentName, subject.Name)
		require.Equal(t, agentNamespace, subject.Namespace)

		require.Equal(t, "rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
		require.Equal(t, "ClusterRole", crb.RoleRef.Kind)
		require.Equal(t, agentName, crb.RoleRef.Name)
	})

	t.Run("should create metrics service", func(t *testing.T) {
		var svcs corev1.ServiceList
		require.NoError(t, client.List(ctx, &svcs))
		require.Len(t, svcs.Items, 1)

		svc := svcs.Items[0]
		require.NotNil(t, svc)
		require.Equal(t, agentName+"-metrics", svc.Name)
		require.Equal(t, agentNamespace, svc.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":                 agentName,
			"telemetry.kyma-project.io/self-monitor": "enabled",
		}, svc.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
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

	t.Run("should create network policy", func(t *testing.T) {
		var nps networkingv1.NetworkPolicyList
		require.NoError(t, client.List(ctx, &nps))
		require.Len(t, nps.Items, 1)

		np := nps.Items[0]
		require.NotNil(t, np)
		require.Equal(t, agentName, np.Name)
		require.Equal(t, agentNamespace, np.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, np.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
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

	t.Run("should create collector config configmap", func(t *testing.T) {
		var cms corev1.ConfigMapList
		require.NoError(t, client.List(ctx, &cms))
		require.Len(t, cms.Items, 1)

		cm := cms.Items[0]
		require.Equal(t, agentName, cm.Name)
		require.Equal(t, agentNamespace, cm.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, cm.Labels)
		require.Contains(t, cm.Data, "relay.conf")
		require.Equal(t, agentCfg, cm.Data["relay.conf"])
	})

	t.Run("should create a daemonset", func(t *testing.T) {
		var dss appsv1.DaemonSetList
		require.NoError(t, client.List(ctx, &dss))
		require.Len(t, dss.Items, 1)

		ds := dss.Items[0]
		require.Equal(t, agentName, ds.Name)
		require.Equal(t, agentNamespace, ds.Namespace)

		// labels
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, ds.Labels, "must have expected daemonset labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": agentName,
		}, ds.Spec.Selector.MatchLabels, "must have expected daemonset selector labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":  agentName,
			"sidecar.istio.io/inject": "true",
		}, ds.Spec.Template.ObjectMeta.Labels, "must have expected pod labels")

		// annotations
		podAnnotations := ds.Spec.Template.ObjectMeta.Annotations
		require.NotEmpty(t, podAnnotations["checksum/config"])
		require.Equal(t, "# configure an env variable OUTPUT_CERTS to write certificates to the given folder\nproxyMetadata:\n  OUTPUT_CERTS: /etc/istio-output-certs\n", podAnnotations["proxy.istio.io/config"])
		require.Equal(t, "[{\"name\": \"istio-certs\", \"mountPath\": \"/etc/istio-output-certs\"}]", podAnnotations["sidecar.istio.io/userVolumeMount"])
		require.Equal(t, "", podAnnotations["traffic.sidecar.istio.io/includeInboundPorts"])
		require.Equal(t, "4317", podAnnotations["traffic.sidecar.istio.io/includeOutboundPorts"])
		require.Equal(t, "8888", podAnnotations["traffic.sidecar.istio.io/excludeInboundPorts"])
		require.Equal(t, "", podAnnotations["traffic.sidecar.istio.io/includeOutboundIPRanges"])

		// collector container
		require.Len(t, ds.Spec.Template.Spec.Containers, 1)
		container := ds.Spec.Template.Spec.Containers[0]

		require.NotNil(t, container.LivenessProbe, "liveness probe must be defined")
		require.NotNil(t, container.ReadinessProbe, "readiness probe must be defined")

		envVars := container.Env
		require.Len(t, envVars, 3)
		require.Equal(t, envVars[0].Name, "MY_POD_IP")
		require.Equal(t, envVars[1].Name, "MY_NODE_NAME")
		require.Equal(t, envVars[2].Name, "GOMEMLIMIT")
		require.Equal(t, envVars[0].ValueFrom.FieldRef.FieldPath, "status.podIP")
		require.Equal(t, envVars[1].ValueFrom.FieldRef.FieldPath, "spec.nodeName")

		// security contexts
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

func TestDeleteAgentResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	sut := AgentApplierDeleter{
		Config: AgentConfig{
			Config: Config{
				BaseName:  agentName,
				Namespace: agentNamespace,
			},
		},
		RBAC: createAgentRBAC(),
	}

	// Create agent resources before testing deletion
	err := sut.ApplyResources(ctx, client, AgentApplyOptions{
		AllowedPorts:        []int32{5555, 6666},
		CollectorConfigYAML: agentCfg,
	})
	require.NoError(t, err)

	// Delete agent resources
	err = sut.DeleteResources(ctx, client)
	require.NoError(t, err)

	t.Run("should delete service account", func(t *testing.T) {
		var serviceAccount corev1.ServiceAccount
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &serviceAccount)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete cluster role", func(t *testing.T) {
		var clusterRole rbacv1.ClusterRole
		err := client.Get(ctx, types.NamespacedName{Name: agentName}, &clusterRole)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete cluster role binding", func(t *testing.T) {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &clusterRoleBinding)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete metrics service", func(t *testing.T) {
		var service corev1.Service
		err := client.Get(ctx, types.NamespacedName{Name: agentName + "-metrics", Namespace: agentNamespace}, &service)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete network policy", func(t *testing.T) {
		var networkPolicy networkingv1.NetworkPolicy
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &networkPolicy)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete collector config configmap", func(t *testing.T) {
		var configMap corev1.ConfigMap
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &configMap)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete daemonset", func(t *testing.T) {
		var daemonSet appsv1.DaemonSet
		err := client.Get(ctx, types.NamespacedName{Name: agentName, Namespace: agentNamespace}, &daemonSet)
		require.True(t, apierrors.IsNotFound(err))
	})
}

func createAgentRBAC() Rbac {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentName,
			Namespace: agentNamespace,
			Labels:    defaultLabels(agentName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"test"},
				Resources: []string{"test"},
				Verbs:     []string{"test"},
			},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentName,
			Namespace: agentNamespace,
			Labels:    defaultLabels(agentName),
		},
		Subjects: []rbacv1.Subject{{Name: agentName, Namespace: agentNamespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     agentName,
		},
	}

	return Rbac{
		clusterRole:        clusterRole,
		clusterRoleBinding: clusterRoleBinding,
		role:               nil,
		roleBinding:        nil,
	}
}
