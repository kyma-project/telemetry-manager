package shared

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
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const (
	// keepOriginalBody = true (default)
	scenarioKeepOriginal = "keep-original-body"
	// keepOriginalBody = false
	scenarioDropOriginal = "drop-original-body"
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
		WithName(pipelineDropOriginalName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(sourceNsKeepOriginal)}...).
		WithKeepOriginalBody(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendKeepOriginal.Endpoint())).
		Build()

	pipelineDropOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(sourceNsDropOriginal)}...).
		WithKeepOriginalBody(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendDropOriginal.Endpoint())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(sourceNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(sourceNsDropOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsDropOriginal).K8sObject(),
		&pipelineDropOriginal,
		&pipelineKeepOriginal,
		loggen.New(sourceNsKeepOriginal).WithUseJSON().K8sObject(),
		loggen.New(sourceNsDropOriginal).WithUseJSON().K8sObject(),
	)
	resources = append(resources, backendKeepOriginal.K8sObjects()...)
	resources = append(resources, backendDropOriginal.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backendKeepOriginal.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backendDropOriginal.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineKeepOriginalName)
	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineDropOriginalName)

	assert.OTelLogsFromNamespaceDelivered(t.Context(), backendDropOriginal, sourceNsKeepOriginal)

	// Scenario [keepOriginalBody=false with JSON logs]: Ship `JSON` Logs without original body
	// Since JSON body is parsed, the original body is not shipped and JSON fields are present in attributes
	// Parse logline with `message` and move to body
	assert.BackendDataEventuallyMatches(t.Context(), backendDropOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "a")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Parse logline with `msg` and move to body
	assert.BackendDataEventuallyMatches(t.Context(), backendDropOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "b")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Parse logline which has `body` attribute
	// Since message or msg is not present the `Body()`` is empty
	assert.BackendDataEventuallyMatches(t.Context(), backendDropOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "c")),
			HaveLogBody(BeEmpty()),
			HaveAttributes(HaveKeyWithValue("body", "c-body")),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Scenario [keepOriginalBody=false with Plain logs]: Ship `Plain` Logs with original body
	// Since Plain body is not parsed, the original body is shipped to the backend and attributes are empty
	assert.BackendDataEventuallyMatches(t.Context(), backendDropOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveLogBody(HavePrefix("name=d")),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Tests where we expect log.Original() to be present
	assert.OTelLogsFromNamespaceDelivered(t.Context(), backendKeepOriginal, sourceNsDropOriginal)

	// Scenario [keepOriginalBody=true with JSON logs]: Ship `JSON` Logs with original body
	assert.BackendDataEventuallyMatches(t.Context(), backendKeepOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "a")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(HaveKey("log.original")),
		)),
	))

	// Parse logline with `msg`
	assert.BackendDataEventuallyMatches(t.Context(), backendKeepOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "b")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(HaveKey("log.original")),
		)),
	))

	// Parse logline which has `body`
	// Since message or msg is not present the `Body()` is empty
	assert.BackendDataEventuallyMatches(t.Context(), backendKeepOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "c")),
			HaveLogBody(BeEmpty()),
			HaveAttributes(HaveKeyWithValue("body", "c-body")),
			HaveAttributes(HaveKey("log.original")),
		)),
	))

	// Scenario [keepOriginalBody=true with Plain logs]: Ship `Plain` Logs with original body
	assert.BackendDataEventuallyMatches(t.Context(), backendKeepOriginal, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveLogBody(HavePrefix("name=d")),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))
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
		WithKeepOriginalBody(false).
		WithHTTPOutput(testutils.HTTPHost(backendKeepOriginal.Host()), testutils.HTTPPort(backendKeepOriginal.Port())).
		Build()

	pipelineDropOriginal := testutils.NewLogPipelineBuilder().
		WithName(pipelineDropOriginalName).
		WithApplicationInput(true).
		WithIncludeNamespaces(sourceNsDropOriginal).
		WithKeepOriginalBody(true).
		WithHTTPOutput(testutils.HTTPHost(backendDropOriginal.Host()), testutils.HTTPPort(backendDropOriginal.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(sourceNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(sourceNsDropOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsKeepOriginal).K8sObject(),
		kitk8s.NewNamespace(backendNsDropOriginal).K8sObject(),
		&pipelineDropOriginal,
		&pipelineKeepOriginal,
		loggen.New(sourceNsKeepOriginal).WithUseJSON().K8sObject(),
		loggen.New(sourceNsDropOriginal).WithUseJSON().K8sObject(),
	)
	resources = append(resources, backendKeepOriginal.K8sObjects()...)
	resources = append(resources, backendDropOriginal.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, backendKeepOriginal.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backendDropOriginal.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineKeepOriginalName)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineDropOriginalName)

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backendDropOriginal, sourceNsKeepOriginal)

	// Scenario [keepOriginalBody=false with JSON logs]: Ship `JSON` Logs without original body
	assert.BackendDataConsistentlyMatches(t.Context(), backendDropOriginal, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("name", "b")),
			fluentbit.HaveLogBody(BeEmpty()),
		)),
	))

	// Scenario [keepOriginalBody=false with Plain logs]: Ship `Plain` Logs with original body shipped to Backend1
	assert.BackendDataEventuallyMatches(t.Context(), backendDropOriginal, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveLogBody(HavePrefix("name=d")),
		)),
	))

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backendKeepOriginal, sourceNsDropOriginal)

	// Scenario [keepOriginalBody=true with JSON logs]: Ship `JSON` Logs with original body shipped to Backend2
	assert.BackendDataEventuallyMatches(t.Context(), backendKeepOriginal, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("name", "b")),
			fluentbit.HaveLogBody(Not(BeEmpty())),
		)),
	))

	// Scenario [keepOriginalBody=false with Plain logs]: Ship `Plain` Logs with original body shipped to Backend2
	assert.BackendDataEventuallyMatches(t.Context(), backendKeepOriginal, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveLogBody(HavePrefix("name=d")),
		)),
	))
}
