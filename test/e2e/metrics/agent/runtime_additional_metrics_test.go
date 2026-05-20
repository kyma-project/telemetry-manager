package agent

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestRuntimeAdditionalMetrics(t *testing.T) {
	suite.SetupTest(t, suite.LabelMetricAgent)

	var (
		uniquePrefix                 = unique.Prefix()
		pipelineName                 = uniquePrefix()
		podWithRequestsAndLimitsName = uniquePrefix()

		backendNs = uniquePrefix("backend")
		includeNs = uniquePrefix("include")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	additionalMetrics := []string{
		"k8s.container.cpu.node.utilization",
		"k8s.container.cpu_limit_utilization",
		"k8s.container.cpu_request_utilization",
		"k8s.container.memory.node.utilization",
		"k8s.container.memory_limit_utilization",
		"k8s.container.memory_request_utilization",
		"k8s.pod.cpu.node.utilization",
		"k8s.pod.cpu_limit_utilization",
		"k8s.pod.cpu_request_utilization",
		"k8s.pod.memory.node.utilization",
		"k8s.pod.memory_limit_utilization",
		"k8s.pod.memory_request_utilization",
	}

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
		WithRuntimeInputAdditionalMetrics(additionalMetrics...).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	podWithRequestsAndLimits := createPodWithRequestsAndLimits(podWithRequestsAndLimitsName, includeNs)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(includeNs).K8sObject(),
		&pipeline,
		podWithRequestsAndLimits,
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	t.Log("Resources should exist and be operational")
	assert.MetricPipelineHealthy(t, pipelineName)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, backend)

	t.Log("Additional metrics should be delivered to the backend")
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ContainElements(additionalMetrics))),
		assert.WithOptionalDescription("Failed to find additional metrics %v", additionalMetrics),
	)
}

func createPodWithRequestsAndLimits(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("64Mi"),
							corev1.ResourceCPU:    resource.MustParse("100m"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("128Mi"),
							corev1.ResourceCPU:    resource.MustParse("200m"),
						},
					},
				},
			},
		},
	}
}
