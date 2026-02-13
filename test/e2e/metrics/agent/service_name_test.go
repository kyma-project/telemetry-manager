package agent

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TODO(TeodorSAP): Remove this test in favor of service_enrichment_test.go once legacy service enrichment strategy is fully deprecated.
func TestServiceName(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricAgentSetB)

	const (
		jobName                                           = "job"
		podWithInvalidStartForUnknownServicePatternName   = "pod-with-invalid-start-for-unknown-service-pattern"
		podWithInvalidEndForUnknownServicePatternName     = "pod-with-invalid-end-for-unknown-service-pattern"
		podWithMissingProcessForUnknownServicePatternName = "pod-with-missing-process-for-unknown-service-pattern"
		attrWithInvalidStartForUnknownServicePattern      = "test_unknown_service"
		attrWithInvalidEndForUnknownServicePattern        = "unknown_service_test"
		attrWithMissingProcessForUnknownServicePattern    = "unknown_service:"
	)

	var (
		uniquePrefix  = unique.Prefix()
		pipelineName  = uniquePrefix()
		daemonSetName = uniquePrefix()
		backendNs     = uniquePrefix("backend")
		genNs         = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(true, testutils.IncludeNamespaces(kitkyma.SystemNamespaceName)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics,
		telemetrygen.WithServiceName(""))
	podSpecWithInvalidStartForUnknownServicePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics,
		telemetrygen.WithServiceName(attrWithInvalidStartForUnknownServicePattern))
	podSpecWithInvalidEndForUnknownServicePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics,
		telemetrygen.WithServiceName(attrWithInvalidEndForUnknownServicePattern))
	podSpecWithMissingProcessForUnknownServicePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics,
		telemetrygen.WithServiceName(attrWithMissingProcessForUnknownServicePattern))

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		kitk8sobjects.NewDaemonSet(daemonSetName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8sobjects.NewJob(jobName, genNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8sobjects.NewPod(podWithInvalidStartForUnknownServicePatternName, genNs).WithPodSpec(podSpecWithInvalidStartForUnknownServicePattern).K8sObject(),
		kitk8sobjects.NewPod(podWithInvalidEndForUnknownServicePatternName, genNs).WithPodSpec(podSpecWithInvalidEndForUnknownServicePattern).K8sObject(),
		kitk8sobjects.NewPod(podWithMissingProcessForUnknownServicePatternName, genNs).WithPodSpec(podSpecWithMissingProcessForUnknownServicePattern).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.MetricPipelineHealthy(t, pipelineName)
	assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)

	verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
		assert.BackendDataEventuallyMatches(t, backend,
			HaveFlatMetrics(
				ContainElement(SatisfyAll(
					HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
					HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
				)),
			),
		)
	}

	verifyServiceNameAttr(daemonSetName, daemonSetName)
	verifyServiceNameAttr(jobName, jobName)

	// Should NOT enrich service.name attribute when its value is not following the unknown_service:<process.executable.name> pattern
	verifyServiceNameAttr(podWithInvalidStartForUnknownServicePatternName, attrWithInvalidStartForUnknownServicePattern)
	verifyServiceNameAttr(podWithInvalidEndForUnknownServicePatternName, attrWithInvalidEndForUnknownServicePattern)
	verifyServiceNameAttr(podWithMissingProcessForUnknownServicePatternName, attrWithMissingProcessForUnknownServicePattern)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			ContainElement(HaveResourceAttributes(HaveKeyWithValue("service.name", names.MetricGateway))),
		), assert.WithOptionalDescription("Should have metrics with service.name set to telemetry-metric-gateway"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			ContainElement(HaveResourceAttributes(HaveKeyWithValue("service.name", names.MetricAgent))),
		), assert.WithOptionalDescription("Should have metrics with service.name set to telemetry-metric-agent"),
	)
}
