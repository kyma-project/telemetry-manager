package shared

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const maxNumberOfLogPipelines = telemetrycontrollers.MaxPipelineCount

func TestMultiPipelineMaxPipeline(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMaxPipeline)

	var (
		uniquePrefix               = unique.Prefix()
		backendNs                  = uniquePrefix("backend")
		generatorNs                = uniquePrefix("gen")
		pipelineBase               = uniquePrefix()
		additionalFBPipelineName   = fmt.Sprintf("%s-limit-exceeding-fb", pipelineBase)
		additionalOTelPipelineName = fmt.Sprintf("%s-limit-exceeding-otel", pipelineBase)
		pipelines                  []client.Object
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	for i := range maxNumberOfLogPipelines {
		pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
		// every other pipeline will have an HTTP output
		var pipeline telemetryv1alpha1.LogPipeline
		if i%2 == 0 {
			// FluentBit pipeline
			pipeline = testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithApplicationInput(true).
				WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
				Build()
		} else {
			// OTel pipeline
			pipeline = testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(testutils.BuildLogPipelineApplicationInput()).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()
		}

		pipelines = append(pipelines, &pipeline)
	}

	additionalFBPipeline := testutils.NewLogPipelineBuilder().
		WithName(additionalFBPipelineName).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	additionalOTelPipeline := testutils.NewLogPipelineBuilder().
		WithName(additionalOTelPipelineName).
		WithInput(testutils.BuildLogPipelineApplicationInput()).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		loggen.New(generatorNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...))            //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, pipelines[2:]...))        //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &additionalFBPipeline))   //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &additionalOTelPipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, pipelines...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	t.Log("Asserting 5 pipelines are healthy")

	for i, pipeline := range pipelines {
		if i%2 == 0 {
			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline.GetName())
		} else {
			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline.GetName())
		}
	}

	t.Log("Attempting to create the 6th pipeline (FluentBit)")
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, &additionalFBPipeline))
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalFBPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalFBPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Deleting one previously healthy pipeline and expecting the additional FluentBit pipeline to be healthy")

	deletePipeline := pipelines[0]
	require.NoError(t, kitk8s.DeleteObjects(t.Context(), suite.K8sClient, deletePipeline))
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, additionalFBPipeline.GetName())

	t.Log("Attempting to create the 6th pipeline (OTel)")
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, &additionalOTelPipeline))
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalOTelPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalOTelPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, generatorNs)

	t.Log("Deleting one previously healthy pipeline and expecting the additional OTel pipeline to be healthy")

	deletePipeline = pipelines[1]
	require.NoError(t, kitk8s.DeleteObjects(t.Context(), suite.K8sClient, deletePipeline))
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, additionalFBPipeline.GetName())
}

func TestMultiPipelineMaxPipeline_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMaxPipelineOTel)

	var (
		uniquePrefix           = unique.Prefix()
		backendNs              = uniquePrefix("backend")
		genNs                  = uniquePrefix("gen")
		pipelineBase           = uniquePrefix()
		additionalPipelineName = fmt.Sprintf("%s-limit-exceeding", pipelineBase)
		pipelines              []client.Object
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

	for i := range maxNumberOfLogPipelines {
		pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
		pipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(genNs))).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		pipelines = append(pipelines, &pipeline)
	}

	additionalPipeline := testutils.NewLogPipelineBuilder().
		WithName(additionalPipelineName).
		WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(genNs))).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		telemetrygen.NewDeployment(genNs, telemetrygen.SignalTypeLogs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...))        //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, pipelines[1:]...))    //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &additionalPipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, pipelines...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)

	t.Log("Asserting 5 pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline.GetName())
	}

	t.Log("Attempting to create the 6th pipeline")
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, &additionalPipeline))
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.OTelLogsFromNamespaceDelivered(t.Context(), backend, genNs)

	t.Log("Deleting one previously healthy pipeline and expecting the additional pipeline to be healthy")

	deletePipeline := pipelines[0]
	require.NoError(t, kitk8s.DeleteObjects(t.Context(), suite.K8sClient, deletePipeline))
	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, additionalPipeline.GetName())
}

func TestMultiPipelineMaxPipeline_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMaxPipelineFluentBit)

	var (
		uniquePrefix           = unique.Prefix()
		backendNs              = uniquePrefix("backend")
		generatorNs            = uniquePrefix("gen")
		pipelineBase           = uniquePrefix()
		additionalPipelineName = fmt.Sprintf("%s-limit-exceeding", pipelineBase)
		pipelines              []client.Object
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	for i := range maxNumberOfLogPipelines {
		pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
		pipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInput(true).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		pipelines = append(pipelines, &pipeline)
	}

	additionalPipeline := testutils.NewLogPipelineBuilder().
		WithName(additionalPipelineName).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		loggen.New(generatorNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...))        //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, pipelines[1:]...))    //nolint:usetesting // Remove ctx from DeleteObjects
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &additionalPipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, pipelines...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	t.Log("Asserting 5 pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline.GetName())
	}

	t.Log("Attempting to create the 6th pipeline")
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, &additionalPipeline))
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, generatorNs)

	t.Log("Deleting one previously healthy pipeline and expecting the additional pipeline to be healthy")

	deletePipeline := pipelines[0]
	require.NoError(t, kitk8s.DeleteObjects(t.Context(), suite.K8sClient, deletePipeline))
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, additionalPipeline.GetName())
}
