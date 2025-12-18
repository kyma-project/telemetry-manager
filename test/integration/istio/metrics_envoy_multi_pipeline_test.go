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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsEnvoyMultiPipeline(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelGardener, suite.LabelIstio)

	var (
		uniquePrefix = unique.Prefix()
		backendNs    = uniquePrefix()
		app1Ns       = uniquePrefix("app-1")
		app2Ns       = uniquePrefix("app-2")
		metricNames  = []string{"envoy_cluster_version", "envoy_cluster_upstream_rq_total", "envoy_cluster_upstream_cx_total"}
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-2"))

	pipelineIncludeApp1Ns := testutils.NewMetricPipelineBuilder().
		WithName("pipeline-envoy").
		WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithIstioInputEnvoyMetrics(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.EndpointHTTP())).
		Build()

	pipelineExcludeApp1Ns := testutils.NewMetricPipelineBuilder().
		WithName("pipeline-non-envoy").
		WithIstioInput(true, testutils.ExcludeNamespaces(app1Ns)).
		WithIstioInputEnvoyMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(app1Ns, kitk8sobjects.WithIstioInjection()).K8sObject(),
		kitk8sobjects.NewNamespace(app2Ns, kitk8sobjects.WithIstioInjection()).K8sObject(),
		&pipelineIncludeApp1Ns,
		&pipelineExcludeApp1Ns,
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)
	resources = append(resources, trafficgen.K8sObjects(app1Ns)...)
	resources = append(resources, trafficgen.K8sObjects(app2Ns)...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)

	assert.BackendDataEventuallyMatches(t, backend1,
		HaveFlatMetrics(
			ContainElement(HaveName(BeElementOf(metricNames))),
		),
	)

	assert.BackendDataEventuallyMatches(t, backend2,
		HaveFlatMetrics(
			Not(ContainElement(HaveName(BeElementOf(metricNames)))),
		),
	)
}
