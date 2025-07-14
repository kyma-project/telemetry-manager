package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	// keepOriginalBody = true (default)
	scenarioKeepOriginal = "keep-original-body"
	// keepOriginalBody = false
	scenarioDropOriginal = "drop-original-body"
)

var logLines = []string{
	`{"scenario": "message", "message":"a-body"}`,
	`{"scenario": "msg", "msg":"b-body"}`,
	`{"scenario": "none", "body":"c-body"}`,
}

func TestKeepOriginalBody_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix         = unique.Prefix()
		sourceNsKeepOriginal = uniquePrefix("source" + scenarioKeepOriginal)
		sourceNsDropOriginal = uniquePrefix("source" + scenarioDropOriginal)

		backendNsKeepOriginal = uniquePrefix("backend" + scenarioKeepOriginal)
		backendNsDropOriginal = uniquePrefix("backend" + scenarioDropOriginal)

		pipelineKeepOriginalName = uniquePrefix(scenarioKeepOriginal)
		pipelineDropOriginalName = uniquePrefix(scenarioDropOriginal)
	)

	backendKeepOriginal := kitbackend.New(backendNsKeepOriginal, kitbackend.SignalTypeLogsOTel, kitbackend.WithName(scenarioKeepOriginal))
	backendDropOriginal := kitbackend.New(backendNsDropOriginal, kitbackend.SignalTypeLogsOTel, kitbackend.WithName(scenarioDropOriginal))

	pipelineKeepOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(sourceNsKeepOriginal)}...).
		WithKeepOriginalBody(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendKeepOriginal.Endpoint())).
		Build()

	pipelineDropOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineDropOriginalName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(sourceNsDropOriginal)}...).
		WithKeepOriginalBody(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendDropOriginal.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(sourceNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(sourceNsDropOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsDropOriginal).K8sObject(),
		&pipelineDropOriginal,
		&pipelineKeepOriginal,
		stdloggen.NewDeployment(
			sourceNsKeepOriginal,
			stdloggen.AppendLogLines(logLines...),
		).K8sObject(),
		stdloggen.NewDeployment(sourceNsDropOriginal,
			stdloggen.AppendLogLines(logLines...),
		).K8sObject(),
	}
	resources = append(resources, backendKeepOriginal.K8sObjects()...)
	resources = append(resources, backendDropOriginal.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(resources...))
	})
	Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())

	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.DeploymentReady(t, backendKeepOriginal.NamespacedName())
	assert.DeploymentReady(t, backendDropOriginal.NamespacedName())
	assert.DaemonSetReady(t, kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t, pipelineKeepOriginalName)
	assert.OTelLogPipelineHealthy(t, pipelineDropOriginalName)

	assert.OTelLogsFromNamespaceDelivered(t, backendDropOriginal, sourceNsDropOriginal)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "message")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		))),
		"Scenario keepOriginalBody=false with JSON logs and 'message' should have attributes, the 'message' moved into the body, and no attribute 'log.original'",
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		))),
		"Scenario keepOriginalBody=false with JSON logs and 'msg' should have attributes, the 'msg' moved into the body, and no attribute 'log.original'",
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "none")),
			HaveLogBody(BeEmpty()),
			HaveAttributes(HaveKeyWithValue("body", "c-body")),
			HaveAttributes(Not(HaveKey("log.original"))),
		))),
		"Scenario keepOriginalBody=false with JSON logs should have attributes, the body empty, and no attribute 'log.original'",
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveLogBody(Equal(stdloggen.DefaultLine)),
			HaveAttributes(Not(HaveKey("log.original"))),
			HaveAttributes(Not(HaveKey("scenario"))),
		))),
		"Scenario keepOriginalBody=false with plain logs should have no attributes, the body filled, and no attribute 'log.original'",
	)

	assert.OTelLogsFromNamespaceDelivered(t, backendKeepOriginal, sourceNsKeepOriginal)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "message")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(HaveKey("log.original")),
		))),
		"Scenario keepOriginalBody=true with JSON logs and 'message' should have attributes, the 'message' moved into the body, and have attribute 'log.original'",
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(HaveKey("log.original")),
		))),
		"Scenario keepOriginalBody=true with JSON logs and 'msg' should have attributes, the 'msg' moved into the body, and have attribute 'log.original'",
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "none")),
			HaveLogBody(BeEmpty()),
			HaveAttributes(HaveKeyWithValue("body", "c-body")),
			HaveAttributes(HaveKey("log.original")),
		))),
		"Scenario keepOriginalBody=true with JSON logs should have attributes, the body empty, and have attribute 'log.original'",
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveLogBody(Equal(stdloggen.DefaultLine)),
			HaveAttributes(Not(HaveKey("log.original"))),
			HaveAttributes(Not(HaveKey("scenario"))),
		))),
		"Scenario keepOriginalBody=true with plain logs should have no attributes, the body filled, and no attribute 'log.original'",
	)
}

func TestKeepOriginalBody_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix         = unique.Prefix()
		sourceNsKeepOriginal = uniquePrefix("source" + scenarioKeepOriginal)
		sourceNsDropOriginal = uniquePrefix("source" + scenarioDropOriginal)

		backendNsKeepOriginal = uniquePrefix("backend" + scenarioKeepOriginal)
		backendNsDropOriginal = uniquePrefix("backend" + scenarioDropOriginal)

		pipelineKeepOriginalName = uniquePrefix(scenarioKeepOriginal)
		pipelineDropOriginalName = uniquePrefix(scenarioDropOriginal)
	)

	backendKeepOriginal := kitbackend.New(backendNsKeepOriginal, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName(scenarioKeepOriginal))
	backendDropOriginal := kitbackend.New(backendNsDropOriginal, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName(scenarioDropOriginal))

	pipelineKeepOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalName).
		WithApplicationInput(true).
		WithIncludeNamespaces(sourceNsKeepOriginal).
		WithKeepOriginalBody(true).
		WithHTTPOutput(testutils.HTTPHost(backendKeepOriginal.Host()), testutils.HTTPPort(backendKeepOriginal.Port())).
		Build()

	pipelineDropOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineDropOriginalName).
		WithApplicationInput(true).
		WithIncludeNamespaces(sourceNsDropOriginal).
		WithKeepOriginalBody(false).
		WithHTTPOutput(testutils.HTTPHost(backendDropOriginal.Host()), testutils.HTTPPort(backendDropOriginal.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(sourceNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(sourceNsDropOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsDropOriginal).K8sObject(),
		&pipelineDropOriginal,
		&pipelineKeepOriginal,
		stdloggen.NewDeployment(
			sourceNsKeepOriginal,
			stdloggen.AppendLogLines(logLines...),
		).K8sObject(),
		stdloggen.NewDeployment(
			sourceNsDropOriginal,
			stdloggen.AppendLogLines(logLines...),
		).K8sObject(),
	}
	resources = append(resources, backendKeepOriginal.K8sObjects()...)
	resources = append(resources, backendDropOriginal.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(resources...))
	})
	Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())

	assert.DeploymentReady(t, backendKeepOriginal.NamespacedName())
	assert.DeploymentReady(t, backendDropOriginal.NamespacedName())
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t, pipelineKeepOriginalName)
	assert.FluentBitLogPipelineHealthy(t, pipelineDropOriginalName)

	assert.FluentBitLogsFromNamespaceDelivered(t, backendDropOriginal, sourceNsDropOriginal)

	assert.BackendDataConsistentlyMatches(t, backendDropOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			fluentbit.HaveLogBody(BeEmpty()),
		))),
		"Scenario keepOriginalBody=false with JSON logs should parse attributes and not have a body",
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(Not(HaveKey("scenario"))),
			fluentbit.HaveLogBody(Equal(stdloggen.DefaultLine)),
		))),
		"Scenario keepOriginalBody=false with plain logs should not have attributes and have a body",
	)

	assert.FluentBitLogsFromNamespaceDelivered(t, backendKeepOriginal, sourceNsKeepOriginal)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			fluentbit.HaveLogBody(Not(BeEmpty())),
		))),
		"Scenario keepOriginalBody=true with JSON logs should parse attributes and have a body",
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(Not(HaveKey("scenario"))),
			fluentbit.HaveLogBody(Equal(stdloggen.DefaultLine)),
		))),
		"Scenario keepOriginalBody=true with plain logs should not have attributes and have a body",
	)
}
