package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsOTLPInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelIstio)

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
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	metricPipelineIstiofiedBackend := testutils.NewMetricPipelineBuilder().
		WithName(pipeline2Name).
		WithOTLPOutput(testutils.OTLPEndpoint(istiofiedBackend.Endpoint())).
		Build()

	peerAuth := kitk8s.NewPeerAuthentication(kitbackend.DefaultName, istiofiedBackendNs)

	podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(istiofiedBackendNs, kitk8s.WithIstioInjection()).K8sObject(),
		&metricPipeline,
		&metricPipelineIstiofiedBackend,
		peerAuth.K8sObject(kitk8s.WithLabel("app", kitbackend.DefaultName)),
		kitk8s.NewDeployment("metric-producer-1", backendNs).WithPodSpec(podSpec).K8sObject(),
		kitk8s.NewDeployment("metric-producer-2", istiofiedBackendNs).WithPodSpec(podSpec).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, istiofiedBackend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())

		for _, resource := range resources {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
				err := suite.K8sClient.Get(suite.Ctx, key, resource)
				g.Expect(err == nil).To(BeFalse())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		}
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.BackendReachable(t, backend)
	assert.BackendReachable(t, istiofiedBackend)

	assert.MetricsFromNamespaceDelivered(t, backend, backendNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, backend, istiofiedBackendNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, istiofiedBackend, backendNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, istiofiedBackend, istiofiedBackendNs, telemetrygen.MetricNames)
}
