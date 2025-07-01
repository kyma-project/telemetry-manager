package agent

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTraceParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(genNs)}...).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		stdloggen.NewDeployment(
			genNs,
			stdloggen.AppendLogLines(
				`{"scenario": "traceIdFullOnly", "trace_id": "255c2212dd02c02ac59a923ff07aec74", "span_id": "c5c735f175ad06a6", "trace_flags": "01"}`,
				`{"scenario": "traceparentOnly", "traceparent": "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01"}`,
				`{"scenario": "traceIdPartialOnly", "span_id": "123456789"}`,
				`{"scenario": "traceIdAndTraceparent", "trace_id": "255c2212dd02c02ac59a923ff07aec74", "span_id": "c5c735f175ad06a6", "traceparent": "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01"}`,
			),
		).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t, pipelineName)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "traceIdFullOnly")),
			HaveTraceID(Equal("255c2212dd02c02ac59a923ff07aec74")),
			HaveSpanID(Equal("c5c735f175ad06a6")),
			HaveTraceFlags(Equal(uint32(1))),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		"Scenario traceIdFullOnly should parse all trace_id attributes and remove them",
	)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "traceparentOnly")),
			HaveTraceID(Equal("80e1afed08e019fc1110464cfa66635c")),
			HaveSpanID(Equal("7a085853722dc6d2")),
			HaveTraceFlags(Equal(uint32(1))),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		"Scenario traceparentOnly should parse the traceparent attribute and remove it",
	)

	assert.BackendDataConsistentlyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "traceIdPartialOnly")),
			HaveTraceID(BeEmpty()),
			HaveSpanID(BeEmpty()),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(HaveKey("span_id")),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		"Scenario traceIdPartialOnly should not parse any trace attribute and keep the span_id",
	)

	assert.BackendDataConsistentlyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "traceIdAndTraceparent")),
			HaveTraceID(Equal("255c2212dd02c02ac59a923ff07aec74")),
			HaveSpanID(Equal("c5c735f175ad06a6")),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes((HaveKeyWithValue("traceparent", "00-80e1afed08e019fc1110464cfa66635c-7a085853722dc6d2-01"))),
		))),
		"Scenario traceIdAndTraceparent should parse trace attributes, and remove them, and keep traceparent attribute",
	)

	assert.BackendDataConsistentlyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveLogBody(Equal(stdloggen.DefaultLine)),
			HaveTraceID(BeEmpty()),
			HaveSpanID(BeEmpty()),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		))),
		"Default scenario should not have any trace data",
	)
}
