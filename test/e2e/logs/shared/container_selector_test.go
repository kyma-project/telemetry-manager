package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestContainerSelector_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix        = unique.Prefix("agent")
		genNs               = uniquePrefix("gen")
		backendNs           = uniquePrefix("backend")
		container1          = "gen-container-1"
		container2          = "gen-container-2"
		includePipelineName = uniquePrefix("include")
		excludePipelineName = uniquePrefix("exclude")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-2"))

	// Include container1 from namespace genNs
	includePipeline := testutils.NewLogPipelineBuilder().
		WithName(includePipelineName).
		WithIncludeContainers(container1).
		WithIncludeNamespaces(genNs).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.EndpointHTTP())).
		Build()

	// Exclude container1 from namespace genNs
	excludePipeline := testutils.NewLogPipelineBuilder().
		WithName(excludePipelineName).
		WithExcludeContainers(container1).
		WithIncludeNamespaces(genNs).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&includePipeline,
		&excludePipeline,
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithContainer(container1)).WithName("gen-1").K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithContainer(container2)).WithName("gen-2").K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.DaemonSetReady(t, kitkyma.LogAgentName)
	assert.OTelLogPipelineHealthy(t, includePipelineName)
	assert.OTelLogPipelineHealthy(t, excludePipelineName)

	// backend1 - only container1 should be delivered
	assert.OTelLogsFromContainerDelivered(t, backend1, container1)
	assert.OTelLogsFromContainerNotDelivered(t, backend1, container2)

	// backend2 - only container2 should be delivered
	assert.OTelLogsFromContainerNotDelivered(t, backend2, container1)
	assert.OTelLogsFromContainerDelivered(t, backend2, container2)
}

func TestContainerSelector_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix        = unique.Prefix()
		genNs               = uniquePrefix("gen")
		backendNs           = uniquePrefix("backend")
		container1          = "gen-container-1"
		container2          = "gen-container-2"
		includePipelineName = uniquePrefix("include")
		excludePipelineName = uniquePrefix("exclude")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-2"))

	includePipeline := testutils.NewLogPipelineBuilder().
		WithName(includePipelineName).
		WithRuntimeInput(true).
		WithIncludeContainers(container1).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	excludePipeline := testutils.NewLogPipelineBuilder().
		WithName(excludePipelineName).
		WithRuntimeInput(true).
		WithExcludeContainers(container1).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&includePipeline,
		&excludePipeline,
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithContainer(container1)).WithName("gen-1").K8sObject(),
		stdoutloggen.NewDeployment(genNs, stdoutloggen.WithContainer(container2)).WithName("gen-2").K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.FluentBitLogPipelineHealthy(t, includePipelineName)
	assert.FluentBitLogPipelineHealthy(t, excludePipelineName)

	// backend1 - only container1 should be delivered
	assert.FluentBitLogsFromContainerDelivered(t, backend1, container1)
	assert.FluentBitLogsFromContainerNotDelivered(t, backend1, container2)

	// backend2 - only container2 should be delivered
	assert.FluentBitLogsFromContainerNotDelivered(t, backend2, container1)
	assert.FluentBitLogsFromContainerDelivered(t, backend2, container2)
}
