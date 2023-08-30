package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	config = Config{
		BaseName:  "collector",
		Namespace: "telemetry-system",
		Service: ServiceConfig{
			OTLPServiceName: "collector-traces",
		},
		Deployment: DeploymentConfig{
			BaseCPULimit:         resource.MustParse(".25"),
			DynamicCPULimit:      resource.MustParse("0"),
			BaseMemoryLimit:      resource.MustParse("1Gi"),
			DynamicMemoryLimit:   resource.MustParse("1Gi"),
			BaseCPURequest:       resource.MustParse(".1"),
			DynamicCPURequest:    resource.MustParse("0"),
			BaseMemoryRequest:    resource.MustParse("100Mi"),
			DynamicMemoryRequest: resource.MustParse("0"),
		},
	}
)

func TestMakeSecret(t *testing.T) {
	secretData := map[string][]byte{
		"BASIC_AUTH_HEADER": []byte("basicAuthHeader"),
		"OTLP_ENDPOINT":     []byte("otlpEndpoint"),
	}
	secret := MakeSecret(config, secretData)

	require.NotNil(t, secret)
	require.Equal(t, secret.Name, config.BaseName)
	require.Equal(t, secret.Namespace, config.Namespace)

	require.Equal(t, "otlpEndpoint", string(secret.Data["OTLP_ENDPOINT"]), "Secret must contain Otlp endpoint")
	require.Equal(t, "basicAuthHeader", string(secret.Data["BASIC_AUTH_HEADER"]), "Secret must contain basic auth header")
}

func TestMakeClusterRole(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-metric-gateway", Namespace: "telemetry-system"}
	clusterRole := MakeClusterRole(name)
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

	require.NotNil(t, clusterRole)
	require.Equal(t, clusterRole.Name, name.Name)
	require.Equal(t, clusterRole.Rules, expectedRules)
}

func TestMakeDeployment(t *testing.T) {
	deployment := MakeDeployment(config, "123", 1, "MY_POD_IP", "MY_NODE_NAME")

	require.NotNil(t, deployment)
	require.Equal(t, deployment.Name, config.BaseName)
	require.Equal(t, deployment.Namespace, config.Namespace)
	require.Equal(t, *deployment.Spec.Replicas, int32(2))

	require.Equal(t, deployment.Spec.Template.ObjectMeta.Labels["app.kubernetes.io/name"], config.BaseName)
	require.Equal(t, deployment.Spec.Template.ObjectMeta.Labels["app.kubernetes.io/name"], config.BaseName)
	require.Equal(t, deployment.Spec.Template.ObjectMeta.Labels["sidecar.istio.io/inject"], "false")
	require.Equal(t, deployment.Spec.Template.ObjectMeta.Annotations["checksum/config"], "123")
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers[0].EnvFrom)

	resources := deployment.Spec.Template.Spec.Containers[0].Resources
	require.Equal(t, config.Deployment.BaseCPURequest, *resources.Requests.Cpu(), "cpu requests should be defined")
	require.Equal(t, config.Deployment.BaseMemoryRequest.Value(), resources.Requests.Memory().Value(), "memory requests should be defined")
	require.Equal(t, config.Deployment.BaseCPULimit, *resources.Limits.Cpu(), "cpu limit should be defined")
	require.True(t, resource.MustParse("2Gi").Equal(*resources.Limits.Memory()), "memory limit should be defined")

	require.NotNil(t, deployment.Spec.Template.Spec.Containers[0].LivenessProbe, "liveness probe must be defined")
	require.NotNil(t, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe, "readiness probe must be defined")

	podSecurityContext := deployment.Spec.Template.Spec.SecurityContext
	require.NotNil(t, podSecurityContext, "pod security context must be defined")
	require.NotZero(t, podSecurityContext.RunAsUser, "must run as non-root")
	require.True(t, *podSecurityContext.RunAsNonRoot, "must run as non-root")

	containerSecurityContext := deployment.Spec.Template.Spec.Containers[0].SecurityContext
	require.NotNil(t, containerSecurityContext, "container security context must be defined")
	require.NotZero(t, containerSecurityContext.RunAsUser, "must run as non-root")
	require.True(t, *containerSecurityContext.RunAsNonRoot, "must run as non-root")
	require.False(t, *containerSecurityContext.Privileged, "must not be privileged")
	require.False(t, *containerSecurityContext.AllowPrivilegeEscalation, "must not escalate to privileged")
	require.True(t, *containerSecurityContext.ReadOnlyRootFilesystem, "must use readonly fs")

	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	require.Len(t, envVars, 2)
	require.Equal(t, envVars[0].Name, "MY_POD_IP")
	require.Equal(t, envVars[1].Name, "MY_NODE_NAME")
	require.Equal(t, envVars[0].ValueFrom.FieldRef.FieldPath, "status.podIP")
	require.Equal(t, envVars[1].ValueFrom.FieldRef.FieldPath, "spec.nodeName")
}

func TestMakeOTLPService(t *testing.T) {
	service := MakeOTLPService(config)

	require.NotNil(t, service)
	require.Equal(t, service.Name, config.Service.OTLPServiceName)
	require.Equal(t, service.Namespace, config.Namespace)

	expectedLabels := map[string]string{"app.kubernetes.io/name": config.BaseName}
	require.Equal(t, service.Spec.Selector, expectedLabels)

	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.NotEmpty(t, service.Spec.Ports)
	require.Len(t, service.Spec.Ports, 2)
	require.Equal(t, service.Spec.SessionAffinity, corev1.ServiceAffinityClientIP)
}

func TestMakeMetricsService(t *testing.T) {
	service := MakeMetricsService(config)

	require.NotNil(t, service)
	require.Equal(t, service.Name, config.BaseName+"-metrics")
	require.Equal(t, service.Namespace, config.Namespace)

	expectedLabels := map[string]string{"app.kubernetes.io/name": config.BaseName}
	require.Equal(t, service.Spec.Selector, expectedLabels)

	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.Len(t, service.Spec.Ports, 1)

	require.Contains(t, service.Annotations, "prometheus.io/scrape")
	require.Contains(t, service.Annotations, "prometheus.io/port")
}

func TestMakeOpenCensusService(t *testing.T) {
	service := MakeOpenCensusService(config)

	require.NotNil(t, service)
	require.Equal(t, service.Name, config.BaseName+"-internal")
	require.Equal(t, service.Namespace, config.Namespace)

	expectedLabels := map[string]string{"app.kubernetes.io/name": config.BaseName}
	require.Equal(t, service.Spec.Selector, expectedLabels)

	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.NotEmpty(t, service.Spec.Ports)
	require.Len(t, service.Spec.Ports, 1)
	require.Equal(t, service.Spec.SessionAffinity, corev1.ServiceAffinityClientIP)
}

func TestMakeResourceRequirements(t *testing.T) {
	requirements := makeResourceRequirements(config, 1)
	require.Equal(t, config.Deployment.BaseCPURequest, *requirements.Requests.Cpu())
	require.Equal(t, config.Deployment.BaseMemoryRequest.Value(), requirements.Requests.Memory().Value())
	require.Equal(t, config.Deployment.BaseCPULimit.Value(), requirements.Limits.Cpu().Value())
	require.True(t, resource.MustParse("2Gi").Equal(*requirements.Limits.Memory()))
}

func TestMultiPipelineMakeResourceRequirements(t *testing.T) {
	requirements := makeResourceRequirements(config, 3)
	require.Equal(t, config.Deployment.BaseCPURequest, *requirements.Requests.Cpu())
	require.Equal(t, config.Deployment.BaseMemoryRequest.Value(), requirements.Requests.Memory().Value())
	require.Equal(t, config.Deployment.BaseCPULimit.Value(), requirements.Limits.Cpu().Value())
	require.True(t, resource.MustParse("4Gi").Equal(*requirements.Limits.Memory()))
}

func TestMakeNetworkPolicy(t *testing.T) {
	testPorts := []intstr.IntOrString{
		{
			Type:   0,
			IntVal: 5000,
			StrVal: "",
		},
	}
	networkPolicy := MakeNetworkPolicy(config, testPorts)

	require.NotNil(t, networkPolicy)
	require.Equal(t, networkPolicy.Name, config.BaseName+"-pprof-deny-ingress")
	require.Equal(t, networkPolicy.Namespace, config.Namespace)

	expectedLabels := map[string]string{"app.kubernetes.io/name": config.BaseName}
	require.Equal(t, networkPolicy.Spec.PodSelector.MatchLabels, expectedLabels)

	require.Len(t, networkPolicy.Spec.PolicyTypes, 1)
	require.Equal(t, networkPolicy.Spec.PolicyTypes[0], networkingv1.PolicyTypeIngress)
	require.Len(t, networkPolicy.Spec.Ingress, 1)
	require.Len(t, networkPolicy.Spec.Ingress[0].From, 1)
	require.Equal(t, networkPolicy.Spec.Ingress[0].From[0].IPBlock.CIDR, "0.0.0.0/0")
	require.Len(t, networkPolicy.Spec.Ingress[0].Ports, 1)
	require.Equal(t, networkPolicy.Spec.Ingress[0].Ports[0].Port.IntVal, int32(5000))
}
