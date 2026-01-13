package agent

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTraceParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix            = unique.Prefix()
		pipelineName            = uniquePrefix()
		backendNs               = uniquePrefix("backend")
		genNs                   = uniquePrefix("gen")
		traceIdFullOnlyScenario = map[string]string{
			"scenario":    "trace-id-full-only",
			"trace_id":    "255c2212dd02c02ac59a923ff07aec74",
			"span_id":     "c5c735f175ad06a6",
			"trace_flags": "01",
		}
		traceparentOnlyScenario = map[string]string{
			"scenario":    "traceparent-only",
			"traceparent": "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01",
		}
		traceIdPartialOnlyScenario = map[string]string{
			"scenario": "trace-id-partial-only",
			"span_id":  "123456789",
		}
		traceIdAndTraceparentScenario = map[string]string{
			"scenario":    "trace-id-and-traceparent",
			"trace_id":    "255c2212dd02c02ac59a923ff07aec74",
			"span_id":     "c5c735f175ad06a6",
			"traceparent": "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01",
		}
		defaultScenario = map[string]string{
			"scenario": "default",
		}
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(true,
			testutils.IncludeNamespaces(genNs)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(traceIdFullOnlyScenario)).WithName(traceIdFullOnlyScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(traceparentOnlyScenario)).WithName(traceparentOnlyScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(traceIdPartialOnlyScenario)).WithName(traceIdPartialOnlyScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(traceIdAndTraceparentScenario)).WithName(traceIdAndTraceparentScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithFields(defaultScenario)).WithName(defaultScenario["scenario"]).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.DaemonSetReady(t, kitkyma.LogAgentName)
	assert.OTelLogPipelineHealthy(t, pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "trace-id-full-only")),
			HaveTraceID(Equal("255c2212dd02c02ac59a923ff07aec74")),
			HaveSpanID(Equal("c5c735f175ad06a6")),
			HaveTraceFlags(Equal(uint32(1))),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		assert.WithOptionalDescription("Scenario trace-id-full-only should parse all trace_id attributes and remove them"),
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "traceparent-only")),
			HaveTraceID(Equal("80e1afed08e019fc1110464cfa66635c")),
			HaveSpanID(Equal("7a085853722dc6d2")),
			HaveTraceFlags(Equal(uint32(1))),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		assert.WithOptionalDescription("Scenario traceparent-only should parse the traceparent attribute and remove it"),
	)

	assert.BackendDataConsistentlyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "trace-id-partial-only")),
			HaveTraceID(BeEmpty()),
			HaveSpanID(BeEmpty()),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(HaveKey("span_id")),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		assert.WithOptionalDescription("Scenario trace-id-partial-only should not parse any trace attribute and keep the span_id"),
	)

	assert.BackendDataConsistentlyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "trace-id-and-traceparent")),
			HaveTraceID(Equal("255c2212dd02c02ac59a923ff07aec74")),
			HaveSpanID(Equal("c5c735f175ad06a6")),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes((HaveKeyWithValue("traceparent", "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01"))),
		))),
		assert.WithOptionalDescription("Scenario trace-id-and-traceparent should parse trace attributes, and remove them, and keep traceparent attribute"),
	)

	assert.BackendDataConsistentlyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "default")),
			HaveTraceID(BeEmpty()),
			HaveSpanID(BeEmpty()),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		assert.WithOptionalDescription("Default scenario should not have any trace data"),
	)
}
