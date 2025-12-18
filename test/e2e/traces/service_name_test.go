package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestServiceName(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	const (
		podWithNoLabelsName              = "pod-with-no-labels"
		podWithUnknownServiceName        = "pod-with-unknown-service"
		podWithUnknownServicePatternName = "pod-with-unknown-service-pattern"
		unknownServiceName               = "unknown_service"
		unknownServicePatternName        = "unknown_service:bash"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(""))
	podSpecWithUnknownService := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(unknownServiceName))
	podSpecWithUnknownServicePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(unknownServicePatternName))

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		kitk8sobjects.NewPod(podWithNoLabelsName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8sobjects.NewPod(podWithUnknownServiceName, genNs).WithPodSpec(podSpecWithUnknownService).K8sObject(),
		kitk8sobjects.NewPod(podWithUnknownServicePatternName, genNs).WithPodSpec(podSpecWithUnknownServicePattern).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
		assert.BackendDataEventuallyMatches(t, backend,
			HaveFlatTraces(ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
				HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
			))),
		)
	}

	verifyServiceNameAttr(podWithNoLabelsName, podWithNoLabelsName)
	verifyServiceNameAttr(podWithUnknownServiceName, podWithUnknownServiceName)
	verifyServiceNameAttr(podWithUnknownServicePatternName, podWithUnknownServicePatternName)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(
			Not(ContainElement(HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))))),
		), assert.WithOptionalDescription("Should have no kyma resource attributes"),
	)
}
