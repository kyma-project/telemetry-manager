package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	istiosecurityv1 "istio.io/api/security/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	gatewayNamespace     = "my-namespace"
	gatewayName          = "my-gateway"
	gatewayCfg           = "dummy otel collector config"
	baseCPURequest       = resource.MustParse("150m")
	dynamicCPURequest    = resource.MustParse("75m")
	baseCPULimit         = resource.MustParse("300m")
	dynamicCPULimit      = resource.MustParse("150m")
	baseMemoryRequest    = resource.MustParse("150m")
	dynamicMemoryRequest = resource.MustParse("75m")
	baseMemoryLimit      = resource.MustParse("300m")
	dynamicMemoryLimit   = resource.MustParse("150m")
	envVars              = map[string][]byte{
		"BASIC_AUTH_HEADER": []byte("basicAuthHeader"),
		"OTLP_ENDPOINT":     []byte("otlpEndpoint"),
	}
	otlpServiceName       = "telemetry"
	replicas        int32 = 3
)

func TestApplyGatewayResources(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	sut := GatewayApplierDeleter{
		Config: createGatewayConfig(),
		RBAC:   createGatewayRBAC(),
	}

	err := sut.ApplyResources(ctx, client, GatewayApplyOptions{
		AllowedPorts:                   []int32{5555, 6666},
		CollectorConfigYAML:            gatewayCfg,
		CollectorEnvVars:               envVars,
		Replicas:                       replicas,
		ResourceRequirementsMultiplier: 1,
	})
	require.NoError(t, err)

	t.Run("should create service account", func(t *testing.T) {
		var sas corev1.ServiceAccountList
		require.NoError(t, client.List(ctx, &sas))
		require.Len(t, sas.Items, 1)

		sa := sas.Items[0]
		require.NotNil(t, sa)
		require.Equal(t, gatewayName, sa.Name)
		require.Equal(t, gatewayNamespace, sa.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, sa.Labels)
	})

	t.Run("should create cluster role", func(t *testing.T) {
		var crs rbacv1.ClusterRoleList
		require.NoError(t, client.List(ctx, &crs))
		require.Len(t, crs.Items, 1)

		cr := crs.Items[0]
		require.NotNil(t, cr)
		require.Equal(t, gatewayName, cr.Name)
		require.Equal(t, gatewayNamespace, cr.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, cr.Labels)
		require.Equal(t, sut.RBAC.clusterRole.Rules, cr.Rules)
	})

	t.Run("should create cluster role binding", func(t *testing.T) {
		var crbs rbacv1.ClusterRoleBindingList
		require.NoError(t, client.List(ctx, &crbs))
		require.Len(t, crbs.Items, 1)

		crb := crbs.Items[0]
		require.NotNil(t, crb)
		require.Equal(t, gatewayName, crb.Name)
		require.Equal(t, gatewayNamespace, crb.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, crb.Labels)

		subject := crb.Subjects[0]
		require.Equal(t, "ServiceAccount", subject.Kind)
		require.Equal(t, gatewayName, subject.Name)
		require.Equal(t, gatewayNamespace, subject.Namespace)

		require.Equal(t, "rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
		require.Equal(t, "ClusterRole", crb.RoleRef.Kind)
		require.Equal(t, gatewayName, crb.RoleRef.Name)
	})

	t.Run("should create role", func(t *testing.T) {
		var rs rbacv1.RoleList
		require.NoError(t, client.List(ctx, &rs))
		require.Len(t, rs.Items, 1)

		r := rs.Items[0]
		require.NotNil(t, r)
		require.Equal(t, gatewayName, r.Name)
		require.Equal(t, gatewayNamespace, r.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, r.Labels)
		require.Equal(t, sut.RBAC.role.Rules, r.Rules)
	})

	t.Run("should create role binding", func(t *testing.T) {
		var rbs rbacv1.RoleBindingList
		require.NoError(t, client.List(ctx, &rbs))
		require.Len(t, rbs.Items, 1)

		rb := rbs.Items[0]
		require.NotNil(t, rb)
		require.Equal(t, gatewayName, rb.Name)
		require.Equal(t, gatewayNamespace, rb.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, rb.Labels)

		subject := rb.Subjects[0]
		require.Equal(t, "ServiceAccount", subject.Kind)
		require.Equal(t, gatewayName, subject.Name)
		require.Equal(t, gatewayNamespace, subject.Namespace)

		require.Equal(t, "rbac.authorization.k8s.io", rb.RoleRef.APIGroup)
		require.Equal(t, "Role", rb.RoleRef.Kind)
		require.Equal(t, gatewayName, rb.RoleRef.Name)
	})

	t.Run("should create metrics service", func(t *testing.T) {
		var svc corev1.Service
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: gatewayName + "-metrics"}, &svc))

		require.NotNil(t, svc)
		require.Equal(t, gatewayName+"-metrics", svc.Name)
		require.Equal(t, gatewayNamespace, svc.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":                 gatewayName,
			"telemetry.kyma-project.io/self-monitor": "enabled",
		}, svc.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
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
		require.Equal(t, gatewayName, np.Name)
		require.Equal(t, gatewayNamespace, np.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, np.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
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

	t.Run("should create env secret", func(t *testing.T) {
		var secrets corev1.SecretList
		require.NoError(t, client.List(ctx, &secrets))
		require.Len(t, secrets.Items, 1)

		secret := secrets.Items[0]
		require.Equal(t, gatewayName, secret.Name)
		require.Equal(t, gatewayNamespace, secret.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, secret.Labels)
		for k, v := range envVars {
			require.Equal(t, v, secret.Data[k])
		}
	})

	t.Run("should create collector config configmap", func(t *testing.T) {
		var cms corev1.ConfigMapList
		require.NoError(t, client.List(ctx, &cms))
		require.Len(t, cms.Items, 1)

		cm := cms.Items[0]
		require.Equal(t, gatewayName, cm.Name)
		require.Equal(t, gatewayNamespace, cm.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, cm.Labels)
		require.Equal(t, gatewayCfg, cm.Data["relay.conf"])
	})

	t.Run("should create a deployment", func(t *testing.T) {
		var deps appsv1.DeploymentList
		require.NoError(t, client.List(ctx, &deps))
		require.Len(t, deps.Items, 1)

		dep := deps.Items[0]
		require.Equal(t, gatewayName, dep.Name)
		require.Equal(t, gatewayNamespace, dep.Namespace)
		require.Equal(t, replicas, *dep.Spec.Replicas)

		// labels
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, dep.Labels, "must have expected deployment labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, dep.Spec.Selector.MatchLabels, "must have expected deployment selector labels")
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":  gatewayName,
			"sidecar.istio.io/inject": "false",
		}, dep.Spec.Template.ObjectMeta.Labels, "must have expected pod labels")

		// annotations
		podAnnotations := dep.Spec.Template.ObjectMeta.Annotations
		require.NotEmpty(t, podAnnotations["checksum/config"])

		// collector container
		require.Len(t, dep.Spec.Template.Spec.Containers, 1)
		container := dep.Spec.Template.Spec.Containers[0]

		require.NotNil(t, container.LivenessProbe, "liveness probe must be defined")
		require.NotNil(t, container.ReadinessProbe, "readiness probe must be defined")
		resources := container.Resources

		CPURequest := baseCPURequest
		CPURequest.Add(dynamicCPURequest)
		require.Equal(t, CPURequest.String(), resources.Requests.Cpu().String(), "cpu requests should be calculated correctly")
		memoryRequest := baseMemoryRequest
		memoryRequest.Add(dynamicMemoryRequest)
		require.Equal(t, memoryRequest.String(), resources.Requests.Memory().String(), "memory requests should be calculated correctly")
		CPULimit := baseCPULimit
		CPULimit.Add(dynamicCPULimit)
		require.Equal(t, CPULimit.String(), resources.Limits.Cpu().String(), "cpu limit should be calculated correctly")
		memoryLimit := baseMemoryLimit
		memoryLimit.Add(dynamicMemoryLimit)
		require.Equal(t, memoryLimit.String(), resources.Limits.Memory().String(), "memory limit should be calculated correctly")

		envVars := container.Env
		require.Len(t, envVars, 3)
		require.Equal(t, envVars[0].Name, "MY_POD_IP")
		require.Equal(t, envVars[1].Name, "MY_NODE_NAME")
		require.Equal(t, envVars[2].Name, "GOMEMLIMIT")
		require.Equal(t, envVars[0].ValueFrom.FieldRef.FieldPath, "status.podIP")
		require.Equal(t, envVars[1].ValueFrom.FieldRef.FieldPath, "spec.nodeName")

		// security contexts
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

	t.Run("should create OTLP service", func(t *testing.T) {
		var svc corev1.Service
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: otlpServiceName}, &svc))

		require.NotNil(t, svc)
		require.Equal(t, otlpServiceName, svc.Name)
		require.Equal(t, gatewayNamespace, svc.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
		}, svc.Labels)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": gatewayName,
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
}

func TestApplyGatewayResourcesWithIstioEnabled(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, istiosecurityclientv1.AddToScheme(scheme))
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	sut := GatewayApplierDeleter{
		Config: createGatewayConfig(),
		RBAC:   createGatewayRBAC(),
	}

	err := sut.ApplyResources(ctx, client, GatewayApplyOptions{
		CollectorConfigYAML: gatewayCfg,
		CollectorEnvVars:    envVars,
		IstioEnabled:        true,
		IstioExcludePorts:   []int32{1111, 2222},
		Replicas:            replicas,
	})
	require.NoError(t, err)

	t.Run("should have permissive peer authentication created", func(t *testing.T) {
		var peerAuth istiosecurityclientv1.PeerAuthentication
		require.NoError(t, client.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: gatewayName}, &peerAuth))

		require.Equal(t, gatewayName, peerAuth.Name)
		require.Equal(t, istiosecurityv1.PeerAuthentication_MutualTLS_PERMISSIVE, peerAuth.Spec.Mtls.Mode)
	})

	t.Run("should have istio enabled with ports excluded", func(t *testing.T) {
		var deps appsv1.DeploymentList
		require.NoError(t, client.List(ctx, &deps))
		require.Len(t, deps.Items, 1)
		dep := deps.Items[0]
		require.Equal(t, gatewayName, dep.Name)
		require.Equal(t, gatewayNamespace, dep.Namespace)
		require.Equal(t, replicas, *dep.Spec.Replicas)

		require.Equal(t, map[string]string{
			"app.kubernetes.io/name":  gatewayName,
			"sidecar.istio.io/inject": "true",
		}, dep.Spec.Template.ObjectMeta.Labels, "must have expected pod labels")

		// annotations
		podAnnotations := dep.Spec.Template.ObjectMeta.Annotations
		require.NotEmpty(t, podAnnotations["checksum/config"])
		require.Equal(t, "TPROXY", podAnnotations["sidecar.istio.io/interceptionMode"])
		require.Equal(t, "1111, 2222", podAnnotations["traffic.sidecar.istio.io/excludeInboundPorts"])
	})
}

func TestDeleteGatewayResources(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, istiosecurityclientv1.AddToScheme(scheme))
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	sut := GatewayApplierDeleter{
		Config: createGatewayConfig(),
		RBAC:   createGatewayRBAC(),
	}

	// Create gateway resources before testing deletion
	err := sut.ApplyResources(ctx, client, GatewayApplyOptions{
		CollectorConfigYAML: gatewayCfg,
		CollectorEnvVars:    envVars,
		IstioEnabled:        true,
		IstioExcludePorts:   []int32{1111, 2222},
		Replicas:            replicas,
	})
	require.NoError(t, err)

	// Delete gateway resources
	err = sut.DeleteResources(ctx, client, true)
	require.NoError(t, err)

	t.Run("should delete service account", func(t *testing.T) {
		var serviceAccount corev1.ServiceAccount
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &serviceAccount)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete cluster role binding", func(t *testing.T) {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &clusterRoleBinding)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete cluster role", func(t *testing.T) {
		var clusterRole rbacv1.ClusterRole
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &clusterRole)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete role binding", func(t *testing.T) {
		var roleBinding rbacv1.RoleBinding
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &roleBinding)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete role", func(t *testing.T) {
		var role rbacv1.Role
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &role)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete metrics service", func(t *testing.T) {
		var service corev1.Service
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName + "-metrics", Namespace: gatewayNamespace}, &service)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete network policy", func(t *testing.T) {
		var networkPolicy networkingv1.NetworkPolicy
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &networkPolicy)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete env secret", func(t *testing.T) {
		var secret corev1.Secret
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &secret)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete collector config configmap", func(t *testing.T) {
		var configMap corev1.ConfigMap
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &configMap)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete deployment", func(t *testing.T) {
		var deployment appsv1.Deployment
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &deployment)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete OTLP service", func(t *testing.T) {
		var service corev1.Service
		err := client.Get(ctx, types.NamespacedName{Name: otlpServiceName, Namespace: gatewayNamespace}, &service)
		require.True(t, apierrors.IsNotFound(err))
	})

	t.Run("should delete permissive peer authentication", func(t *testing.T) {
		var peerAuth istiosecurityclientv1.PeerAuthentication
		err := client.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, &peerAuth)
		require.True(t, apierrors.IsNotFound(err))
	})
}

func createGatewayConfig() GatewayConfig {
	return GatewayConfig{
		Config: Config{
			BaseName:  gatewayName,
			Namespace: gatewayNamespace,
		},
		OTLPServiceName: otlpServiceName,

		Deployment: DeploymentConfig{
			BaseCPURequest:       baseCPURequest,
			DynamicCPURequest:    dynamicCPURequest,
			BaseCPULimit:         baseCPULimit,
			DynamicCPULimit:      dynamicCPULimit,
			BaseMemoryRequest:    baseMemoryRequest,
			DynamicMemoryRequest: dynamicMemoryRequest,
			BaseMemoryLimit:      baseMemoryLimit,
			DynamicMemoryLimit:   dynamicMemoryLimit,
		},
	}
}

func createGatewayRBAC() Rbac {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: gatewayNamespace,
			Labels:    defaultLabels(gatewayName),
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
			Name:      gatewayName,
			Namespace: gatewayNamespace,
			Labels:    defaultLabels(gatewayName),
		},
		Subjects: []rbacv1.Subject{{Name: gatewayName, Namespace: gatewayNamespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     gatewayName,
		},
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: gatewayNamespace,
			Labels:    defaultLabels(gatewayName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"test"},
				Resources: []string{"test"},
				Verbs:     []string{"test"},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: gatewayNamespace,
			Labels:    defaultLabels(gatewayName),
		},
		Subjects: []rbacv1.Subject{{Name: gatewayName, Namespace: gatewayNamespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     gatewayName,
		},
	}

	return Rbac{
		clusterRole:        clusterRole,
		clusterRoleBinding: clusterRoleBinding,
		role:               role,
		roleBinding:        roleBinding,
	}
}
