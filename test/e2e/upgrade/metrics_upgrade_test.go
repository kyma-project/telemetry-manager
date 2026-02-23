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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestMetricsUpgrade validates that MetricPipeline resources survive a manager upgrade.
//
// Test flow:
// 1. SetupTest deploys the OLD version (from UPGRADE_FROM_CHART env var, or latest release if not set)
// 2. Create pipeline, backend, and generator resources
// 3. Validate everything works with old version
// 4. Call UpgradeToTargetVersion() to upgrade to MANAGER_IMAGE
// 5. Validate everything still works after upgrade
func TestMetricsUpgrade(t *testing.T) {
	labels := []string{suite.LabelUpgrade, suite.LabelMetrics, suite.LabelOtel}

	// Deploy old version (defaults to latest release if UPGRADE_FROM_CHART not set)
	suite.SetupTestWithOptions(t, labels, kubeprep.WithChartVersion(os.Getenv("UPGRADE_FROM_CHART")))

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	// Create resources (without automatic cleanup - upgrade tests preserve resources)
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	// === VALIDATE OLD VERSION ===
	t.Log("Validating metric pipeline with old version...")
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipelineName)
	assert.BackendReachable(t, backend)
	assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)

	// === UPGRADE TO NEW VERSION ===
	t.Log("Upgrading manager to target version...")
	Expect(suite.UpgradeToTargetVersion(t, labels)).To(Succeed())

	// === VALIDATE AFTER UPGRADE ===
	t.Log("Validating metric pipeline after upgrade...")
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipelineName)
	assert.BackendReachable(t, backend)
	assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)
}
