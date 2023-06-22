package otelcollector

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
			CPULimit:      resource.MustParse(".25"),
			MemoryLimit:   resource.MustParse("400Mi"),
			CPURequest:    resource.MustParse(".1"),
			MemoryRequest: resource.MustParse("100Mi"),
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

func TestMakeDeployment(t *testing.T) {
	deployment := MakeDeployment(config, "123")
	labels := makeDefaultLabels(config)

	require.NotNil(t, deployment)
	require.Equal(t, deployment.Name, config.BaseName)
	require.Equal(t, deployment.Namespace, config.Namespace)
	require.Equal(t, *deployment.Spec.Replicas, int32(1))
	require.Equal(t, deployment.Spec.Selector.MatchLabels, labels)
	require.Equal(t, deployment.Spec.Template.ObjectMeta.Labels, labels)
	for k, v := range defaultPodAnnotations {
		require.Equal(t, deployment.Spec.Template.ObjectMeta.Annotations[k], v)
	}
	require.Equal(t, deployment.Spec.Template.ObjectMeta.Annotations[configHashAnnotationKey], "123")
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers[0].EnvFrom)

	resources := deployment.Spec.Template.Spec.Containers[0].Resources
	require.Equal(t, config.Deployment.CPURequest, *resources.Requests.Cpu(), "cpu requests should be defined")
	require.Equal(t, config.Deployment.MemoryRequest, *resources.Requests.Memory(), "memory requests should be defined")
	require.Equal(t, config.Deployment.CPULimit, *resources.Limits.Cpu(), "cpu limit should be defined")
	require.Equal(t, config.Deployment.MemoryLimit, *resources.Limits.Memory(), "memory limit should be defined")

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
}

func TestMakeOTLPService(t *testing.T) {
	service := MakeOTLPService(config)
	labels := makeDefaultLabels(config)

	require.NotNil(t, service)
	require.Equal(t, service.Name, config.Service.OTLPServiceName)
	require.Equal(t, service.Namespace, config.Namespace)
	require.Equal(t, service.Spec.Selector, labels)
	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.NotEmpty(t, service.Spec.Ports)
	require.Len(t, service.Spec.Ports, 2)
}

func TestMakeMetricsService(t *testing.T) {
	service := MakeMetricsService(config)
	labels := makeDefaultLabels(config)

	require.NotNil(t, service)
	require.Equal(t, service.Name, config.BaseName+"-metrics")
	require.Equal(t, service.Namespace, config.Namespace)
	require.Equal(t, service.Spec.Selector, labels)
	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.Len(t, service.Spec.Ports, 1)

	require.Contains(t, service.Annotations, "prometheus.io/scrape")
	require.Contains(t, service.Annotations, "prometheus.io/port")
}

func TestMakeOpenCensusService(t *testing.T) {
	service := MakeOpenCensusService(config)
	labels := makeDefaultLabels(config)

	require.NotNil(t, service)
	require.Equal(t, service.Name, config.BaseName+"-internal")
	require.Equal(t, service.Namespace, config.Namespace)
	require.Equal(t, service.Spec.Selector, labels)
	require.Equal(t, service.Spec.Type, corev1.ServiceTypeClusterIP)
	require.NotEmpty(t, service.Spec.Ports)
	require.Len(t, service.Spec.Ports, 1)
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
	labels := makeDefaultLabels(config)

	require.NotNil(t, networkPolicy)
	require.Equal(t, networkPolicy.Name, config.BaseName+"-pprof-deny-ingress")
	require.Equal(t, networkPolicy.Namespace, config.Namespace)
	require.Equal(t, networkPolicy.Spec.PodSelector.MatchLabels, labels)
	require.Len(t, networkPolicy.Spec.PolicyTypes, 1)
	require.Equal(t, networkPolicy.Spec.PolicyTypes[0], networkingv1.PolicyTypeIngress)
	require.Len(t, networkPolicy.Spec.Ingress, 1)
	require.Len(t, networkPolicy.Spec.Ingress[0].From, 1)
	require.Equal(t, networkPolicy.Spec.Ingress[0].From[0].IPBlock.CIDR, "0.0.0.0/0")
	require.Len(t, networkPolicy.Spec.Ingress[0].Ports, 1)
	require.Equal(t, networkPolicy.Spec.Ingress[0].Ports[0].Port.IntVal, int32(5000))
}
