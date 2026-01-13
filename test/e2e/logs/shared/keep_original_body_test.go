package shared

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
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	// keepOriginalBody = true (default)
	scenarioKeepOriginal = "keep-original-body"
	// keepOriginalBody = false
	scenarioDropOriginal = "drop-original-body"
	// plaintext to be logged by the stdout log generator
	plaintextLog = "hello world"
)

var (
	messageScenario = map[string]string{
		"scenario": "message",
		"message":  "a-body",
	}
	msgScenario = map[string]string{
		"scenario": "msg",
		"msg":      "b-body",
	}
	noneScenario = map[string]string{
		"scenario": "none",
		"body":     "c-body",
	}
)

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
		WithRuntimeInput(true,
			testutils.IncludeNamespaces(sourceNsKeepOriginal)).
		WithKeepOriginalBody(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendKeepOriginal.EndpointHTTP())).
		Build()

	pipelineDropOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineDropOriginalName).
		WithRuntimeInput(true,
			testutils.IncludeNamespaces(sourceNsDropOriginal)).
		WithKeepOriginalBody(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendDropOriginal.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(sourceNsKeepOriginal).K8sObject(),
		kitk8sobjects.NewNamespace(sourceNsDropOriginal).K8sObject(),
		kitk8sobjects.NewNamespace(backendNsKeepOriginal).K8sObject(),
		kitk8sobjects.NewNamespace(backendNsDropOriginal).K8sObject(),
		&pipelineDropOriginal,
		&pipelineKeepOriginal,
		// stdout log generators in the "keep-original-body" namespace
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithFields(messageScenario)).WithName(messageScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithFields(msgScenario)).WithName(msgScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithFields(noneScenario)).WithName(noneScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithPlaintextFormat(), stdoutloggen.WithText(plaintextLog)).K8sObject(),
		// stdout log generators in the "drop-original-body" namespace
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithFields(messageScenario)).WithName(messageScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithFields(msgScenario)).WithName(msgScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithFields(noneScenario)).WithName(noneScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithPlaintextFormat(), stdoutloggen.WithText(plaintextLog)).K8sObject(),
	}
	resources = append(resources, backendKeepOriginal.K8sObjects()...)
	resources = append(resources, backendDropOriginal.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backendKeepOriginal)
	assert.BackendReachable(t, backendDropOriginal)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
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
		assert.WithOptionalDescription("Scenario keepOriginalBody=false with JSON logs and 'message' should have attributes, the 'message' moved into the body, and no attribute 'log.original'"),
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=false with JSON logs and 'msg' should have attributes, the 'msg' moved into the body, and no attribute 'log.original'"),
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "none")),
			HaveLogBody(BeEmpty()),
			HaveAttributes(HaveKeyWithValue("body", "c-body")),
			HaveAttributes(Not(HaveKey("log.original"))),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=false with JSON logs should have attributes, the body empty, and no attribute 'log.original'"),
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveLogBody(Equal(plaintextLog)),
			HaveAttributes(Not(HaveKey("log.original"))),
			HaveAttributes(Not(HaveKey("scenario"))),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=false with plain logs should have no attributes, the body filled, and no attribute 'log.original'"),
	)

	assert.OTelLogsFromNamespaceDelivered(t, backendKeepOriginal, sourceNsKeepOriginal)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "message")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(HaveKey("log.original")),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=true with JSON logs and 'message' should have attributes, the 'message' moved into the body, and have attribute 'log.original'"),
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(HaveKey("log.original")),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=true with JSON logs and 'msg' should have attributes, the 'msg' moved into the body, and have attribute 'log.original'"),
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("scenario", "none")),
			HaveLogBody(BeEmpty()),
			HaveAttributes(HaveKeyWithValue("body", "c-body")),
			HaveAttributes(HaveKey("log.original")),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=true with JSON logs should have attributes, the body empty, and have attribute 'log.original'"),
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		HaveFlatLogs(ContainElement(SatisfyAll(
			HaveLogBody(Equal(plaintextLog)),
			HaveAttributes(Not(HaveKey("log.original"))),
			HaveAttributes(Not(HaveKey("scenario"))),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=true with plain logs should have no attributes, the body filled, and no attribute 'log.original'"),
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
		WithRuntimeInput(true).
		WithIncludeNamespaces(sourceNsKeepOriginal).
		WithKeepOriginalBody(true).
		WithHTTPOutput(testutils.HTTPHost(backendKeepOriginal.Host()), testutils.HTTPPort(backendKeepOriginal.Port())).
		Build()

	pipelineDropOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineDropOriginalName).
		WithRuntimeInput(true).
		WithIncludeNamespaces(sourceNsDropOriginal).
		WithKeepOriginalBody(false).
		WithHTTPOutput(testutils.HTTPHost(backendDropOriginal.Host()), testutils.HTTPPort(backendDropOriginal.Port())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(sourceNsKeepOriginal).K8sObject(),
		kitk8sobjects.NewNamespace(sourceNsDropOriginal).K8sObject(),
		kitk8sobjects.NewNamespace(backendNsKeepOriginal).K8sObject(),
		kitk8sobjects.NewNamespace(backendNsDropOriginal).K8sObject(),
		&pipelineDropOriginal,
		&pipelineKeepOriginal,
		// stdout log generators in the "keep-original-body" namespace
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithFields(messageScenario)).WithName(messageScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithFields(msgScenario)).WithName(msgScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithFields(noneScenario)).WithName(noneScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsKeepOriginal, stdoutloggen.WithPlaintextFormat(), stdoutloggen.WithText(plaintextLog)).K8sObject(),
		// stdout log generators in the "drop-original-body" namespace
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithFields(messageScenario)).WithName(messageScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithFields(msgScenario)).WithName(msgScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithFields(noneScenario)).WithName(noneScenario["scenario"]).K8sObject(),
		stdoutloggen.NewDeployment(sourceNsDropOriginal, stdoutloggen.WithPlaintextFormat(), stdoutloggen.WithText(plaintextLog)).K8sObject(),
	}
	resources = append(resources, backendKeepOriginal.K8sObjects()...)
	resources = append(resources, backendDropOriginal.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backendKeepOriginal)
	assert.BackendReachable(t, backendDropOriginal)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, pipelineKeepOriginalName)
	assert.FluentBitLogPipelineHealthy(t, pipelineDropOriginalName)
	assert.FluentBitLogsFromNamespaceDelivered(t, backendDropOriginal, sourceNsDropOriginal)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			fluentbit.HaveLogBody(BeEmpty()),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=false with JSON logs should parse attributes and not have a body"),
	)

	assert.BackendDataEventuallyMatches(t, backendDropOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(Not(HaveKey("scenario"))),
			fluentbit.HaveLogBody(Equal(plaintextLog)),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=false with plain logs should not have attributes and have a body"),
	)

	assert.FluentBitLogsFromNamespaceDelivered(t, backendKeepOriginal, sourceNsKeepOriginal)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("scenario", "msg")),
			fluentbit.HaveLogBody(Not(BeEmpty())),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=true with JSON logs should parse attributes and have a body"),
	)

	assert.BackendDataEventuallyMatches(t, backendKeepOriginal,
		fluentbit.HaveFlatLogs(ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(Not(HaveKey("scenario"))),
			fluentbit.HaveLogBody(Equal(plaintextLog)),
		))),
		assert.WithOptionalDescription("Scenario keepOriginalBody=true with plain logs should not have attributes and have a body"),
	)
}
