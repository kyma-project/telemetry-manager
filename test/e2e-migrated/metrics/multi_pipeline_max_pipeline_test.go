package metrics

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
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

func TestMultiPipelineMaxPipeline(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMaxPipeline)

	const maxNumberOfMetricPipelines = telemetrycontrollers.MaxPipelineCount

	var (
		uniquePrefix = unique.Prefix("metrics")
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		pipelineBase           = uniquePrefix()
		additionalPipelineName = fmt.Sprintf("%s-limit-exceeding", pipelineBase)
		pipelines              []client.Object
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	for i := range maxNumberOfMetricPipelines {
		pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
		pipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		pipelines = append(pipelines, &pipeline)
	}

	additionalPipeline := testutils.NewMetricPipelineBuilder().
		WithName(additionalPipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...))        //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), pipelines[1:]...))    //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), &additionalPipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))
	require.NoError(t, kitk8s.CreateObjects(t.Context(), pipelines...))

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)

	t.Log("Asserting 5 pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.MetricPipelineHealthy(t.Context(), pipeline.GetName())
	}

	t.Log("Attempting to create the 6th pipeline")
	require.NoError(t, kitk8s.CreateObjects(t.Context(), &additionalPipeline))
	assert.MetricPipelineHasCondition(t, additionalPipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.MetricPipelineHasCondition(t, additionalPipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)

	t.Log("Deleting one previously healthy pipeline and expecting the additional pipeline to be healthy")

	deletePipeline := pipelines[0]
	require.NoError(t, kitk8s.DeleteObjects(t.Context(), deletePipeline))
	assert.MetricPipelineHealthy(t.Context(), additionalPipeline.GetName())
}
