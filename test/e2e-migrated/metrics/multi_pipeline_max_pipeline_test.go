package metrics

import (
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
	suite.RegisterTestCase(t, suite.LabelMetricsMaxPipeline)

	const maxNumberOfMetricPipelines = telemetrycontrollers.MaxPipelineCount

	var (
		uniquePrefix = unique.Prefix("metrics")
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		pipelineBase           = uniquePrefix()
		additionalPipelineName = fmt.Sprintf("%s-limit-exceeded", pipelineBase)
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
		require.NoError(t, kitk8s.DeleteObjects(resources...))
		require.NoError(t, kitk8s.DeleteObjects(pipelines[1:]...))
		require.NoError(t, kitk8s.DeleteObjects(&additionalPipeline))
	})
	require.NoError(t, kitk8s.CreateObjects(t, resources...))
	require.NoError(t, kitk8s.CreateObjects(t, pipelines...))

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)

	t.Log("Asserting all pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.MetricPipelineHealthy(t, pipeline.GetName())
	}

	t.Log("Attempting to create a pipeline that exceeds the maximum allowed number of pipelines")
	require.NoError(t, kitk8s.CreateObjects(t, &additionalPipeline))
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
	require.NoError(t, kitk8s.DeleteObjects(deletePipeline))
	assert.MetricPipelineHealthy(t, additionalPipeline.GetName())
}
