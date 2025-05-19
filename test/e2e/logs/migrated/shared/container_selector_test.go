package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestContainerSelector_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix                  = unique.Prefix("agent")
		genNs                         = uniquePrefix("gen")
		backendNs                     = uniquePrefix("backend")
		container1                    = "gen-container-1"
		container2                    = "gen-container-2"
		includeContainer1PipelineName = uniquePrefix("include")
		excludeContainer1PipelineName = uniquePrefix("exclude")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-1"))
	backend1ExportURL := backend1.ExportURL(suite.ProxyClient)

	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("backend-2"))
	backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

	includeContainer1Pipeline := testutils.NewLogPipelineBuilder().
		WithName(includeContainer1PipelineName).
		WithIncludeContainers(container1).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
		Build()

	excludeContainer1Pipeline := testutils.NewLogPipelineBuilder().
		WithName(excludeContainer1PipelineName).
		WithExcludeContainers(container1).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&includeContainer1Pipeline,
		&excludeContainer1Pipeline,
		loggen.New(genNs).WithName("gen-1").WithContainer(container1).K8sObject(),
		loggen.New(genNs).WithName("gen-2").WithContainer(container2).K8sObject(),
	)
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())

	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)

	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, includeContainer1PipelineName)
	assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, excludeContainer1PipelineName)

	// backend1 - only container1 should be delivered
	assert.OTelLogsFromContainerDelivered(suite.ProxyClient, backend1ExportURL, container1)
	assert.OTelLogsFromContainerNotDelivered(suite.ProxyClient, backend1ExportURL, container2)

	// backend2 - only container2 should be delivered
	assert.OTelLogsFromContainerNotDelivered(suite.ProxyClient, backend2ExportURL, container1)
	assert.OTelLogsFromContainerDelivered(suite.ProxyClient, backend2ExportURL, container2)
}

func TestContainerSelector_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix                  = unique.Prefix()
		genNs                         = uniquePrefix("gen")
		backendNs                     = uniquePrefix("backend")
		container1                    = "gen-container-1"
		container2                    = "gen-container-2"
		includeContainer1PipelineName = uniquePrefix("include")
		excludeContainer1PipelineName = uniquePrefix("exclude")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-1"))
	backend1ExportURL := backend1.ExportURL(suite.ProxyClient)

	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName("backend-2"))
	backend2ExportURL := backend2.ExportURL(suite.ProxyClient)

	includeContainer1Pipeline := testutils.NewLogPipelineBuilder().
		WithName(includeContainer1PipelineName).
		WithApplicationInput(true).
		WithIncludeContainers(container1).
		WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
		Build()

	excludeContainer1Pipeline := testutils.NewLogPipelineBuilder().
		WithName(excludeContainer1PipelineName).
		WithApplicationInput(true).
		WithExcludeContainers(container1).
		WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
		Build()

	var resources []client.Object
	resources = append(resources,
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&includeContainer1Pipeline,
		&excludeContainer1Pipeline,
		loggen.New(genNs).WithName("gen-1").WithContainer(container1).K8sObject(),
		loggen.New(genNs).WithName("gen-2").WithContainer(container2).K8sObject(),
	)
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend1.NamespacedName())
	assert.DeploymentReady(t.Context(), suite.K8sClient, backend2.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, includeContainer1PipelineName)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, excludeContainer1PipelineName)

	// backend1 - only container1 should be delivered
	assert.FluentBitLogsFromContainerDelivered(suite.ProxyClient, backend1ExportURL, container1)
	assert.FluentBitLogsFromContainerNotDelivered(suite.ProxyClient, backend1ExportURL, container2)

	// backend2 - only container2 should be delivered
	assert.FluentBitLogsFromContainerNotDelivered(suite.ProxyClient, backend2ExportURL, container1)
	assert.FluentBitLogsFromContainerDelivered(suite.ProxyClient, backend2ExportURL, container2)
}
