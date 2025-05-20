package agent

import (
	"context"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestPayloadParser(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		genNs        = uniquePrefix("generator")
		backendNs    = uniquePrefix("backend")
		pipelineName = uniquePrefix()
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(genNs)}...).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		loggen.New(genNs).WithUseJSON().K8sObject(),
		&pipeline,
	)

	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)

	// Parse traces properly
	assert.DataEventuallyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "a")),
			HaveTraceId(Equal("255c2212dd02c02ac59a923ff07aec74")),
			HaveSpanId(Equal("c5c735f175ad06a6")),
			HaveTraceFlags(Equal(uint32(0))),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		)),
	))

	assert.DataEventuallyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "b")),
			HaveTraceId(Equal("80e1afed08e019fc1110464cfa66635c")),
			HaveSpanId(Equal("7a085853722dc6d2")),
			HaveTraceFlags(Equal(uint32(1))),
			HaveAttributes(Not(HaveKey("trace_id"))),
			HaveAttributes(Not(HaveKey("span_id"))),
			HaveAttributes(Not(HaveKey("trace_flags"))),
			HaveAttributes(Not(HaveKey("traceparent"))),
		)),
	))

	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "c")),
			HaveTraceId(BeEmpty()),
			HaveSpanId(BeEmpty()),
			HaveTraceFlags(Equal(uint32(0))), // default value
			HaveAttributes(HaveKey("span_id")),
		)),
	))

	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveLogRecordBody(HavePrefix("name=d")),
			HaveTraceId(BeEmpty()),
			HaveSpanId(BeEmpty()),
			HaveTraceFlags(Equal(uint32(0))), // default value
		)),
	))

	// Parse severity properly
	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "a")),
			HaveSeverityNumber(Equal(9)),
			HaveSeverityText(Equal("INFO")),
			HaveAttributes(Not(HaveKey("level"))),
		)),
	))
	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "b")),
			HaveSeverityNumber(Equal(13)),
			HaveSeverityText(Equal("WARN")),
			HaveAttributes(Not(HaveKey("log.level"))),
		)),
	))

	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "c")),
			HaveSeverityNumber(Equal(0)), // default value
			HaveSeverityText(BeEmpty()),
		)),
	))

	assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
		ContainElement(SatisfyAll(
			HaveLogRecordBody(HavePrefix("name=d")),
			HaveSeverityNumber(Equal(0)), // default value
			HaveSeverityText(BeEmpty()),
		)),
	))
}
