package shared

import (
	"context"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestKeepOriginalBody_OTel(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix = unique.Prefix()
		gen1Ns       = uniquePrefix("gen-1")
		gen2Ns       = uniquePrefix("gen-2")

		backendGen1Ns = uniquePrefix("backend-gen-1")
		backendGen2Ns = uniquePrefix("backend-gen-2")

		pipelineKeepOriginalBodyName        = uniquePrefix("true")
		pipelineWithoutKeepOriginalBodyName = uniquePrefix("false")
	)

	backendGen1 := backend.New(backendGen1Ns, backend.SignalTypeLogsOTel, backend.WithName("backend-gen-1"))
	backendGen1ExportURL := backendGen1.ExportURL(suite.ProxyClient)

	backendGen2 := backend.New(backendGen2Ns, backend.SignalTypeLogsOTel, backend.WithName("backend-gen-2"))
	backendGen2ExportURL := backendGen2.ExportURL(suite.ProxyClient)

	pipelineKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalBodyName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(gen1Ns)}...).
		WithKeepOriginalBody(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backendGen1.Endpoint())).
		Build()

	pipelineWithoutKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineWithoutKeepOriginalBodyName).
		WithApplicationInput(true,
			[]testutils.ExtendedNamespaceSelectorOptions{testutils.ExtIncludeNamespaces(gen2Ns)}...).
		WithKeepOriginalBody(false).
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

	assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendGen1ExportURL, gen1Ns)
	assert.DataConsistentlyMatching(suite.ProxyClient, backendGen1ExportURL, HaveFlatOTelLogs(
		ContainElement(
			HaveAttributes(HaveKey("log.original")),
		),
	))

	assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendGen2ExportURL, gen2Ns)
	assert.DataConsistentlyMatching(suite.ProxyClient, backendGen2ExportURL, HaveFlatOTelLogs(
		Not(ContainElement(
			HaveAttributes(HaveKey("log.original")),
		)),
	))

}

func TestKeepOriginalBody_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix = unique.Prefix()
		gen1Ns       = uniquePrefix("gen-1")
		gen2Ns       = uniquePrefix("gen-2")

		backendGen1Ns = uniquePrefix("backend-gen-1")
		backendGen2Ns = uniquePrefix("backend-gen-2")

		pipelineKeepOriginalBodyName        = uniquePrefix("keep-original-body")
		pipelineWithoutKeepOriginalBodyName = uniquePrefix("without-keep-original-body")
	)

	backendGen1 := backend.New(backendGen1Ns, backend.SignalTypeLogsFluentBit, backend.WithName("backend-gen-1"))
	backendGen1ExportURL := backendGen1.ExportURL(suite.ProxyClient)

	backendGen2 := backend.New(backendGen2Ns, backend.SignalTypeLogsFluentBit, backend.WithName("backend-gen-2"))
	backendGen2ExportURL := backendGen2.ExportURL(suite.ProxyClient)

	pipelineKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineKeepOriginalBodyName).
		WithApplicationInput(true).
		WithIncludeNamespaces(gen1Ns).
		WithKeepOriginalBody(true).
		WithHTTPOutput(testutils.HTTPHost(backendGen1.Host()), testutils.HTTPPort(backendGen1.Port())).
		Build()

	pipelineWithoutKeepOriginalBody := testutils.NewLogPipelineBuilder().
		WithName(pipelineWithoutKeepOriginalBodyName).
		WithApplicationInput(true).
		WithIncludeNamespaces(gen2Ns).
		WithKeepOriginalBody(false).
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

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendGen1ExportURL, gen1Ns)
	assert.DataConsistentlyMatching(suite.ProxyClient, backendGen1ExportURL, HaveFlatFluentBitLogs(
		HaveEach(
			HaveLogBody(Not(BeEmpty())),
		),
	))

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendGen2ExportURL, gen2Ns)
	assert.DataConsistentlyMatching(suite.ProxyClient, backendGen2ExportURL, HaveFlatFluentBitLogs(
		HaveEach(
			HaveLogBody(BeEmpty()),
		),
	))
}
