package upgrade

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestLogsUpgrade validates that LogPipeline resources (OTel) survive a manager upgrade.
//
// Test flow:
// 1. SetupTest deploys the OLD version (from UPGRADE_FROM_CHART env var, or latest release if not set)
// 2. Create pipeline, backend, and generator resources
// 3. Validate everything works with old version
// 4. Call UpgradeToTargetVersion() to upgrade to MANAGER_IMAGE
// 5. Validate everything still works after upgrade
func TestLogsUpgrade(t *testing.T) {
	labels := []string{suite.LabelUpgrade, suite.LabelOtel, suite.LabelLogs}

	// Deploy old version (defaults to latest release if UPGRADE_FROM_CHART not set)
	suite.SetupTestWithOptions(t, labels, kubeprep.WithChartVersion(os.Getenv("UPGRADE_FROM_CHART")))

	var (
		uniquePrefix      = unique.Prefix()
		pipelineName      = uniquePrefix()
		pipelineNameAfter = pipelineName + "-after"
		backendNs         = uniquePrefix("backend")
		backendNsAfter    = uniquePrefix("backend-after")
		genNs             = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		new(pipeline),
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	// Create resources (without automatic cleanup - upgrade tests preserve resources)
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// === VALIDATE OLD VERSION ===
	t.Log("Validating log pipeline with old version...")
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.OTelLogPipelineHealthy(t, pipelineName)
	assert.BackendReachable(t, backend)
	assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

	// === UPGRADE TO NEW VERSION ===
	t.Log("Upgrading manager to target version...")
	Expect(suite.UpgradeToTargetVersion(t)).To(Succeed())

	// === VALIDATE AFTER UPGRADE ===

	// Existing pipelines
	t.Log("Validating existing log pipeline after upgrade...")
	assert.DaemonSetReady(t, kitkyma.TelemetryOTLPGatewayName)
	assert.OTelLogPipelineHealthy(t, pipelineName)
	assert.BackendReachable(t, backend)
	assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

	// Create new pipelines

	backendAfter := kitbackend.New(backendNsAfter, kitbackend.SignalTypeLogsOTel)

	pipelineAfter := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameAfter).
		WithOTLPOutput(testutils.OTLPEndpoint(backendAfter.EndpointHTTP())).
		Build()

	afterResources := []client.Object{
		new(pipelineAfter),
		kitk8sobjects.NewNamespace(backendNsAfter).K8sObject(),
	}
	afterResources = append(afterResources, backendAfter.K8sObjects()...)

	// Create resources (without automatic cleanup - upgrade tests preserve resources)
	Expect(kitk8s.CreateObjects(t, afterResources...)).To(Succeed())

	// ==== NEW PIPELINES ====
	t.Log("Validating log pipeline creation after upgrade...")
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.OTelLogPipelineHealthy(t, pipelineNameAfter)
	assert.BackendReachable(t, backendAfter)
	assert.OTelLogsFromNamespaceDelivered(t, backendAfter, genNs)
}
