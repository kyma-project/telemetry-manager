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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNoInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	var (
		uniquePrefix          = unique.Prefix()
		pipelineNameNoInput   = uniquePrefix("pipeline-no-input")
		pipelineNameWithInput = uniquePrefix("pipeline-with-input")
		backendNs             = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	pipelineNoInput := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameNoInput).
		WithPrometheusInput(false).
		WithRuntimeInput(false).
		WithIstioInput(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	pipelineWithInput := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameWithInput).
		WithPrometheusInput(true).
		WithRuntimeInput(true).
		WithIstioInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&pipelineNoInput,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())
	Expect(kitk8s.CreateObjects(t.Context(), &pipelineWithInput)).Should(Succeed())

	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.MetricPipelineHealthy(t.Context(), pipelineNameNoInput)
	assert.MetricPipelineHealthy(t.Context(), pipelineNameWithInput)

	assert.MetricPipelineHasCondition(t, pipelineNameNoInput, metav1.Condition{
		Type:   conditions.TypeAgentHealthy,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonMetricAgentNotRequired,
	})

	assert.DaemonSetReady(t.Context(), kitkyma.MetricAgentName)
	Expect(kitk8s.DeleteObjects(t.Context(), &pipelineWithInput)).Should(Succeed())
	assert.DaemonSetNotFound(t.Context(), kitkyma.MetricAgentName)
}
