package upgrade

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// MetricPipeline upgrade test flow
func TestMetricsUpgrade(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelUpgrade)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Run("before upgrade", func(t *testing.T) {
		require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

		assert.DeploymentReady(t.Context(), backend.NamespacedName())
		assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)

		assert.MetricPipelineHealthy(t.Context(), pipelineName)
		assert.MetricsFromNamespaceDeliveredWithT(t, backend, genNs, telemetrygen.MetricNames)
	})

	t.Run("after upgrade", func(t *testing.T) {
		assert.DeploymentReady(t.Context(), backend.NamespacedName())
		assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)

		assert.MetricPipelineHealthy(t.Context(), pipelineName)
		assert.MetricsFromNamespaceDeliveredWithT(t, backend, genNs, telemetrygen.MetricNames)
	})
}
