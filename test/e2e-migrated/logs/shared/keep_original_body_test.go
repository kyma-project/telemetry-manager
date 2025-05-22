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

func TestKeepOriginalBody_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		gen1Ns       = uniquePrefix("gen-1")
		gen2Ns       = uniquePrefix("gen-2")

		backendGen1Ns = uniquePrefix("backend-gen-1")
		backendGen2Ns = uniquePrefix("backend-gen-2")

		pipelineKeepOriginalBodyName        = uniquePrefix("keep-original-body")
		pipelineWithoutKeepOriginalBodyName = uniquePrefix("without-keep-original-body")
	)

	backendGen1 := kitbackend.New(backendGen1Ns, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-gen-1"))
	backendGen2 := kitbackend.New(backendGen2Ns, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-gen-2"))

	pipelineWithoutKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineWithoutKeepOriginalBodyName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(gen1Ns)}...).
		WithKeepOriginalBody(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendGen1.Endpoint())).
		Build()

	pipelineKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalBodyName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(gen2Ns)}...).
		WithKeepOriginalBody(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendGen2.Endpoint())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(gen1Ns).K8sObject(),
		kitk8s.NewNamespace(gen2Ns).K8sObject(),
		kitk8s.NewNamespace(backendGen1Ns).K8sObject(),
		kitk8s.NewNamespace(backendGen2Ns).K8sObject(),
		&pipelineKeepOriginalBody,
		&pipelineWithoutKeepOriginalBody,
		loggen.New(gen1Ns).WithUseJSON().K8sObject(),
		loggen.New(gen2Ns).WithUseJSON().K8sObject(),
	)
	resources = append(resources, backendGen1.K8sObjects()...)
	resources = append(resources, backendGen2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backendGen1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backendGen2.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineKeepOriginalBodyName)
	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineWithoutKeepOriginalBodyName)

	assert.OTelLogsFromNamespaceDelivered(t.Context(), backendGen1, gen1Ns)

	// Scenario [keepOriginalBody=false with JSON logs]: Ship `JSON` Logs without original body shipped to Backend1
	// Since JSON body is parsed, the original body is not shipped to the backend and JSON fields are present in attributes
	// Parse logline with `message`
	assert.BackendDataEventuallyMatches(t.Context(), backendGen1, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "a")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Parse logline with `msg`
	assert.BackendDataEventuallyMatches(t.Context(), backendGen1, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "b")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Parse logline which has `body`
	// Since message or msg is not present we keep the `Body()`
	assert.BackendDataEventuallyMatches(t.Context(), backendGen1, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "c")),
			HaveLogBody(Equal("{\"name\": \"c\", \"age\": 30, \"city\": \"Munich\", \"span_id\": \"123456789\", \"body\":\"c-body\"}")),
			HaveAttributes(HaveKey("body")),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Scenario [keepOriginalBody=false with Plain logs]: Ship `Plain` Logs with original body shipped to Backend1
	// Since Plain body is not parsed, the original body is shipped to the backend and attributes is empty
	assert.BackendDataEventuallyMatches(t.Context(), backendGen1, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveLogBody(HavePrefix("name=d")),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Tests where we expect log.Original() to be present
	assert.OTelLogsFromNamespaceDelivered(t.Context(), backendGen2, gen2Ns)

	// Scenario [keepOriginalBody=true with JSON logs]: Ship `JSON` Logs with original body shipped to Backend2
	assert.BackendDataConsistentlyMatches(t.Context(), backendGen2, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "a")),
			HaveLogBody(Equal("a-body")),
			HaveAttributes(Not(HaveKey("message"))),
			HaveAttributes(HaveKey("log.original")),
		)),
	))

	// Parse logline with `msg`
	assert.BackendDataEventuallyMatches(t.Context(), backendGen1, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "b")),
			HaveLogBody(Equal("b-body")),
			HaveAttributes(Not(HaveKey("msg"))),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Parse logline which has `body`
	// Since message or msg is not present we keep the `Body()`
	assert.BackendDataEventuallyMatches(t.Context(), backendGen1, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveAttributes(HaveKeyWithValue("name", "c")),
			HaveLogBody(Equal("{\"name\": \"c\", \"age\": 30, \"city\": \"Munich\", \"span_id\": \"123456789\", \"body\":\"c-body\"}")),
			HaveAttributes(HaveKey("body")),
			HaveAttributes(Not(HaveKey("log.original"))),
		)),
	))

	// Scenario [keepOriginalBody=true with Plain logs]: Ship `Plain` Logs with original body shipped to Backend2
	assert.BackendDataConsistentlyMatches(t.Context(), backendGen2, HaveFlatLogs(
		ContainElement(SatisfyAll(
			HaveLogBody(HavePrefix("name=d")),
		)),
	))
}

func TestKeepOriginalBody_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		gen1Ns       = uniquePrefix("gen-1")
		gen2Ns       = uniquePrefix("gen-2")

		backendGen1Ns = uniquePrefix("backend-gen-1")
		backendGen2Ns = uniquePrefix("backend-gen-2")

		pipelineKeepOriginalBodyName        = uniquePrefix("keep-original-body")
		pipelineWithoutKeepOriginalBodyName = uniquePrefix("without-keep-original-body")
	)

	backendGen1 := kitbackend.New(backendGen1Ns, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-gen-1"))
	backendGen2 := kitbackend.New(backendGen2Ns, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-gen-2"))

	pipelineWithoutKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalBodyName).
		WithApplicationInput(true).
		WithIncludeNamespaces(gen1Ns).
		WithKeepOriginalBody(false).
		WithHTTPOutput(testutils.HTTPHost(backendGen1.Host()), testutils.HTTPPort(backendGen1.Port())).
		Build()

	pipelineKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineWithoutKeepOriginalBodyName).
		WithApplicationInput(true).
		WithIncludeNamespaces(gen2Ns).
		WithKeepOriginalBody(true).
		WithHTTPOutput(testutils.HTTPHost(backendGen2.Host()), testutils.HTTPPort(backendGen2.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(gen1Ns).K8sObject(),
		kitk8s.NewNamespace(gen2Ns).K8sObject(),
		kitk8s.NewNamespace(backendGen1Ns).K8sObject(),
		kitk8s.NewNamespace(backendGen2Ns).K8sObject(),
		&pipelineKeepOriginalBody,
		&pipelineWithoutKeepOriginalBody,
		loggen.New(gen1Ns).WithUseJSON().K8sObject(),
		loggen.New(gen2Ns).WithUseJSON().K8sObject(),
	)
	resources = append(resources, backendGen1.K8sObjects()...)
	resources = append(resources, backendGen2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, backendGen1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backendGen2.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineKeepOriginalBodyName)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineWithoutKeepOriginalBodyName)

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backendGen1, gen1Ns)

	// Scenario [keepOriginalBody=false with JSON logs]: Ship `JSON` Logs without original body shipped to Backend1
	assert.BackendDataConsistentlyMatches(t.Context(), backendGen1, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("name", "b")),
			fluentbit.HaveLogBody(BeEmpty()),
		)),
	))

	// Scenario [keepOriginalBody=false with Plain logs]: Ship `Plain` Logs with original body shipped to Backend1
	assert.BackendDataConsistentlyMatches(t.Context(), backendGen1, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveLogBody(HavePrefix("name=d")),
		)),
	))

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backendGen2, gen2Ns)

	// Scenario [keepOriginalBody=true with JSON logs]: Ship `JSON` Logs with original body shipped to Backend2
	assert.BackendDataConsistentlyMatches(t.Context(), backendGen2, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveAttributes(HaveKeyWithValue("name", "b")),
			fluentbit.HaveLogBody(Not(BeEmpty())),
		)),
	))

	// Scenario [keepOriginalBody=false with Plain logs]: Ship `Plain` Logs with original body shipped to Backend2
	assert.BackendDataConsistentlyMatches(t.Context(), backendGen2, fluentbit.HaveFlatLogs(
		ContainElement(SatisfyAll(
			fluentbit.HaveLogBody(HavePrefix("name=d")),
		)),
	))
}
