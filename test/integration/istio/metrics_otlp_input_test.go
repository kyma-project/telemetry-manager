package istio

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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsOTLPInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelGardener, suite.LabelIstio)

	var (
		uniquePrefix       = unique.Prefix()
		pipeline1Name      = uniquePrefix("1")
		pipeline2Name      = uniquePrefix("2")
		backendNs          = uniquePrefix("backend")
		istiofiedBackendNs = uniquePrefix("backend-istiofied")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	istiofiedBackend := kitbackend.New(istiofiedBackendNs, kitbackend.SignalTypeMetrics)

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipeline1Name).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	metricPipelineIstiofiedBackend := testutils.NewMetricPipelineBuilder().
		WithName(pipeline2Name).
		WithOTLPOutput(testutils.OTLPEndpoint(istiofiedBackend.EndpointHTTP())).
		Build()

	peerAuth := kitk8sobjects.NewPeerAuthentication(kitbackend.DefaultName, istiofiedBackendNs)

	podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(istiofiedBackendNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		&metricPipeline,
		&metricPipelineIstiofiedBackend,
		peerAuth.K8sObject(kitk8sobjects.WithLabel("app", kitbackend.DefaultName)),
		kitk8sobjects.NewDeployment("metric-producer-1", backendNs).WithPodSpec(podSpec).K8sObject(),
		kitk8sobjects.NewDeployment("metric-producer-2", istiofiedBackendNs).WithPodSpec(podSpec).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, istiofiedBackend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.BackendReachable(t, backend)
	assert.BackendReachable(t, istiofiedBackend)

	assert.MetricsFromNamespaceDelivered(t, backend, backendNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, backend, istiofiedBackendNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, istiofiedBackend, backendNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, istiofiedBackend, istiofiedBackendNs, telemetrygen.MetricNames)
}
