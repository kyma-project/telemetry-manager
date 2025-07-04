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

func TestDisabledInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithPrometheusInput(false).
		WithRuntimeInput(false).
		WithIstioInput(false).
		WithOTLPInput(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.MetricPipelineHealthy(t.Context(), pipelineName)
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.BackendReachable(t, backend)

	// If Runtime input is disabled, THEN the metric agent must not be deployed
	assert.DaemonSetNotFound(t.Context(), kitkyma.MetricAgentName)
	assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeAgentHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonMetricAgentNotRequired,
	})

	// If OTLP input is disabled, THEN the metrics pushed to the gateway should not be sent to the backend
	assert.MetricsFromNamespaceNotDelivered(t, backend, genNs)
}
