package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	istiosecurityv1beta "istio.io/api/security/v1beta1"
	istiosecurityclientv1beta "istio.io/client-go/pkg/apis/security/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	namespace         = "my-namespace"
	name              = "my-gateway"
	cfg               = "dummy otel collector config"
	baseCPURequest    = resource.MustParse("150m")
	baseCPULimit      = resource.MustParse("300m")
	baseMemoryRequest = resource.MustParse("150m")
	baseMemoryLimit   = resource.MustParse("300m")
	envVars           = map[string][]byte{
		"BASIC_AUTH_HEADER": []byte("basicAuthHeader"),
		"OTLP_ENDPOINT":     []byte("otlpEndpoint"),
	}
	otlpServiceName       = "telemetry"
	replicas        int32 = 3
)

func TestApplyGatewayResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	err := ApplyGatewayResources(ctx, client, createGatewayConfig(false))
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
		require.Equal(t, cfg, cm.Data["relay.conf"])
	})

	t.Run("should create env var secrets", func(t *testing.T) {
		var secrets corev1.SecretList
		require.NoError(t, client.List(ctx, &secrets))
		require.Len(t, secrets.Items, 1)

		secret := secrets.Items[0]
		require.Equal(t, name, secret.Name)
		require.Equal(t, namespace, secret.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, secret.Labels)
		for k, v := range envVars {
			require.Equal(t, v, secret.Data[k])
		}
	})

	t.Run("should create a deployment", func(t *testing.T) {
		var deps appsv1.DeploymentList
		require.NoError(t, client.List(ctx, &deps))
		require.Len(t, deps.Items, 1)

		dep := deps.Items[0]
		require.Equal(t, name, dep.Name)
		require.Equal(t, namespace, dep.Namespace)
		require.Equal(t, replicas, *dep.Spec.Replicas)

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

		//collector container
		require.Len(t, dep.Spec.Template.Spec.Containers, 1)
		container := dep.Spec.Template.Spec.Containers[0]

		require.NotNil(t, container.LivenessProbe, "liveness probe must be defined")
		require.NotNil(t, container.ReadinessProbe, "readiness probe must be defined")
		resources := container.Resources
		require.Equal(t, baseCPURequest, *resources.Requests.Cpu(), "cpu requests should be defined")
		require.Equal(t, baseMemoryRequest, *resources.Requests.Memory(), "memory requests should be defined")
		require.Equal(t, baseCPULimit, *resources.Limits.Cpu(), "cpu limit should be defined")
		require.Equal(t, baseMemoryLimit, *resources.Limits.Memory(), "memory limit should be defined")

		envVars := container.Env
		require.Len(t, envVars, 2)
		require.Equal(t, envVars[0].Name, "MY_POD_IP")
		require.Equal(t, envVars[1].Name, "MY_NODE_NAME")
		require.Equal(t, envVars[0].ValueFrom.FieldRef.FieldPath, "status.podIP")
		require.Equal(t, envVars[1].ValueFrom.FieldRef.FieldPath, "spec.nodeName")

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
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
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
		var svc corev1.Service
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name + "-metrics"}, &svc))

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

	t.Run("should create otlp service", func(t *testing.T) {
		var svc corev1.Service
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: otlpServiceName}, &svc))

		require.NotNil(t, svc)
		require.Equal(t, otlpServiceName, svc.Name)
		require.Equal(t, namespace, svc.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, svc.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, svc.Spec.Selector)
		require.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
		require.Len(t, svc.Spec.Ports, 2)
		require.Equal(t, corev1.ServicePort{
			Name:       "grpc-collector",
			Protocol:   corev1.ProtocolTCP,
			Port:       4317,
			TargetPort: intstr.FromInt32(4317),
		}, svc.Spec.Ports[0])
		require.Equal(t, corev1.ServicePort{
			Name:       "http-collector",
			Protocol:   corev1.ProtocolTCP,
			Port:       4318,
			TargetPort: intstr.FromInt32(4318),
		}, svc.Spec.Ports[1])
	})

	t.Run("should create open census service", func(t *testing.T) {
		var svc corev1.Service
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name + "-internal"}, &svc))

		require.NotNil(t, svc)
		require.Equal(t, name+"-internal", svc.Name)
		require.Equal(t, namespace, svc.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, svc.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, svc.Spec.Selector)
		require.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
		require.Len(t, svc.Spec.Ports, 1)
		require.Equal(t, corev1.ServicePort{
			Name:       "http-opencensus",
			Protocol:   corev1.ProtocolTCP,
			Port:       55678,
			TargetPort: intstr.FromInt32(55678),
		}, svc.Spec.Ports[0])
	})

}
func TestApplyGatewayResourcesWithIstioEnabled(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, istiosecurityclientv1beta.AddToScheme(scheme))
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := ApplyGatewayResources(ctx, client, createGatewayConfig(true))
	require.NoError(t, err)

	t.Run("It should have permissive peer authentication created", func(t *testing.T) {
		var peerAuth istiosecurityclientv1beta.PeerAuthentication
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &peerAuth))

		require.Equal(t, name, peerAuth.Name)
		require.Equal(t, istiosecurityv1beta.PeerAuthentication_MutualTLS_PERMISSIVE, peerAuth.Spec.Mtls.Mode)
	})

	t.Run("It should have istio enabled with ports excluded", func(t *testing.T) {
		var deps appsv1.DeploymentList
		require.NoError(t, client.List(ctx, &deps))
		require.Len(t, deps.Items, 1)
		dep := deps.Items[0]
		require.Equal(t, name, dep.Name)
		require.Equal(t, namespace, dep.Namespace)
		require.Equal(t, replicas, *dep.Spec.Replicas)

		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":  name,
			"sidecar.istio.io/inject": "true",
		}, dep.Spec.Template.ObjectMeta.Labels, "must have expected pod labels")

		//annotations
		podAnnotations := dep.Spec.Template.ObjectMeta.Annotations
		require.NotEmpty(t, podAnnotations["checksum/config"])
		require.Equal(t, "TPROXY", podAnnotations["sidecar.istio.io/interceptionMode"])
		require.Equal(t, "1111, 2222", podAnnotations["traffic.sidecar.istio.io/excludeInboundPorts"])
	})
}

func createGatewayConfig(istioEnabled bool) *GatewayConfig {
	return &GatewayConfig{
		Config: Config{
			BaseName:         name,
			Namespace:        namespace,
			CollectorConfig:  cfg,
			CollectorEnvVars: envVars,
		},
		OTLPServiceName:      otlpServiceName,
		CanReceiveOpenCensus: true,
		allowedPorts:         []int32{5555, 6666},
		Istio: IstioConfig{
			Enabled:      istioEnabled,
			ExcludePorts: "1111, 2222",
		},
		Scaling: GatewayScalingConfig{
			Replicas: replicas,
		},
		Deployment: DeploymentConfig{
			BaseCPURequest:    baseCPURequest,
			BaseCPULimit:      baseCPULimit,
			BaseMemoryRequest: baseMemoryRequest,
			BaseMemoryLimit:   baseMemoryLimit,
		},
	}
}
