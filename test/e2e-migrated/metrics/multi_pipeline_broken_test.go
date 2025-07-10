package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetB)

	var (
		uniquePrefix        = unique.Prefix()
		healthyPipelineName = uniquePrefix("healthy")
		brokenPipelineName  = uniquePrefix("broken")
		backendNs           = uniquePrefix("backend")
		genNs               = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	healthyPipeline := testutils.NewMetricPipelineBuilder().
		WithName(healthyPipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	brokenPipeline := testutils.NewMetricPipelineBuilder().
		WithName(brokenPipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret("dummy", "dummy", "dummy")). // broken pipeline ref
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&healthyPipeline,
		&brokenPipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t.Context(), healthyPipelineName)

	assert.MetricPipelineHasCondition(t, brokenPipeline.Name, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})
	assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)
}
