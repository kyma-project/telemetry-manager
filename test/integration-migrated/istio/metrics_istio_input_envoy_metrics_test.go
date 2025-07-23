package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestIstioInputEnvoyMetrics(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelIstio)

	var (
		uniquePrefix     = unique.Prefix()
		mockNs           = uniquePrefix()
		app1Ns           = uniquePrefix("app-1")
		pipelineName     = uniquePrefix()
		envoyMetricNames = []string{
			"envoy_cluster_version",
			"envoy_cluster_upstream_rq_total",
			"envoy_cluster_upstream_cx_total",
		}
	)

	backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithIstioInputEnvoyMetrics(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(mockNs).K8sObject(),
		kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject(),
		&metricPipeline,
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, trafficgen.K8sObjects(app1Ns)...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			ContainElement(HaveName(BeElementOf(envoyMetricNames))),
		),
	)
}
