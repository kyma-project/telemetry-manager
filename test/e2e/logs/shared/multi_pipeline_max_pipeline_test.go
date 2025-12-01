package shared

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const maxNumberOfLogPipelines = telemetrycontrollers.MaxPipelineCount

func TestMultiPipelineMaxPipeline(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogsMaxPipeline)

	var (
		uniquePrefix = unique.Prefix("logs")
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		pipelineBase               = uniquePrefix()
		additionalFBPipelineName   = fmt.Sprintf("%s-limit-exceeded-fb", pipelineBase)
		additionalOTelPipelineName = fmt.Sprintf("%s-limit-exceeded-otel", pipelineBase)
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
		objects.NewNamespace(backendNs).K8sObject(),
		objects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
		Expect(kitk8s.DeleteObjects(pipelines[2:]...)).To(Succeed())
		Expect(kitk8s.DeleteObjects(&additionalFBPipeline)).To(Succeed())
		Expect(kitk8s.DeleteObjects(&additionalOTelPipeline)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())
	Expect(kitk8s.CreateObjects(t, pipelines...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)

	t.Log("Asserting all pipelines are healthy")

	for i, pipeline := range pipelines {
		if i%2 == 0 {
			assert.FluentBitLogPipelineHealthy(t, pipeline.GetName())
		} else {
			assert.OTelLogPipelineHealthy(t, pipeline.GetName())
		}
	}

	t.Log("Attempting to create a FluentBit pipeline that exceeds the maximum allowed number of pipelines")
	Expect(kitk8s.CreateObjects(t, &additionalFBPipeline)).To(Succeed())
	assert.LogPipelineHasCondition(t, additionalFBPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t, additionalFBPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Deleting one previously healthy pipeline and expecting the additional FluentBit pipeline to be healthy")

	deletePipeline := pipelines[0]
	Expect(kitk8s.DeleteObjects(deletePipeline)).To(Succeed())
	assert.FluentBitLogPipelineHealthy(t, additionalFBPipeline.GetName())

	t.Log("Attempting to create a OTel pipeline that exceeds the maximum allowed number of pipelines")
	Expect(kitk8s.CreateObjects(t, &additionalOTelPipeline)).To(Succeed())
	assert.LogPipelineHasCondition(t, additionalOTelPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t, additionalOTelPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)

	t.Log("Deleting one previously healthy pipeline and expecting the additional OTel pipeline to be healthy")

	deletePipeline = pipelines[1]
	Expect(kitk8s.DeleteObjects(deletePipeline)).To(Succeed())
	assert.FluentBitLogPipelineHealthy(t, additionalFBPipeline.GetName())
}

func TestMultiPipelineMaxPipeline_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelOTelMaxPipeline)

	var (
		uniquePrefix = unique.Prefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		pipelineBase           = uniquePrefix()
		additionalPipelineName = fmt.Sprintf("%s-limit-exceeded", pipelineBase)
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
		objects.NewNamespace(backendNs).K8sObject(),
		objects.NewNamespace(genNs).K8sObject(),
		telemetrygen.NewDeployment(genNs, telemetrygen.SignalTypeLogs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
		Expect(kitk8s.DeleteObjects(pipelines[1:]...)).To(Succeed())
		Expect(kitk8s.DeleteObjects(&additionalPipeline)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())
	Expect(kitk8s.CreateObjects(t, pipelines...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)

	t.Log("Asserting all pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.OTelLogPipelineHealthy(t, pipeline.GetName())
	}

	t.Log("Attempting to create a pipeline that exceeds the maximum allowed number of pipelines")
	Expect(kitk8s.CreateObjects(t, &additionalPipeline)).To(Succeed())
	assert.LogPipelineHasCondition(t, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

	t.Log("Deleting one previously healthy pipeline and expecting the additional pipeline to be healthy")

	deletePipeline := pipelines[0]
	Expect(kitk8s.DeleteObjects(deletePipeline)).To(Succeed())
	assert.OTelLogPipelineHealthy(t, additionalPipeline.GetName())
}

func TestMultiPipelineMaxPipeline_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBitMaxPipeline)

	var (
		uniquePrefix = unique.Prefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		pipelineBase           = uniquePrefix()
		additionalPipelineName = fmt.Sprintf("%s-limit-exceeded", pipelineBase)
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
		objects.NewNamespace(backendNs).K8sObject(),
		objects.NewNamespace(genNs).K8sObject(),
		stdoutloggen.NewDeployment(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
		Expect(kitk8s.DeleteObjects(pipelines[1:]...)).To(Succeed())
		Expect(kitk8s.DeleteObjects(&additionalPipeline)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())
	Expect(kitk8s.CreateObjects(t, pipelines...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)

	t.Log("Asserting all pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.FluentBitLogPipelineHealthy(t, pipeline.GetName())
	}

	t.Log("Attempting to create a pipeline that exceeds the maximum allowed number of pipelines")
	Expect(kitk8s.CreateObjects(t, &additionalPipeline)).To(Succeed())
	assert.LogPipelineHasCondition(t, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonMaxPipelinesExceeded,
	})
	assert.LogPipelineHasCondition(t, additionalPipeline.GetName(), metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.FluentBitLogsFromNamespaceDelivered(t, backend, genNs)

	t.Log("Deleting one previously healthy pipeline and expecting the additional pipeline to be healthy")

	deletePipeline := pipelines[0]
	Expect(kitk8s.DeleteObjects(deletePipeline)).To(Succeed())
	assert.FluentBitLogPipelineHealthy(t, additionalPipeline.GetName())
}
