package istio

import (
	"io"
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
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsIstioEnvoyMultiPipeline(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelIstio)

	var (
		uniquePrefix = unique.Prefix()
		backendNs    = uniquePrefix()
		app1Ns       = uniquePrefix("app-1")
		app2Ns       = uniquePrefix("app-2")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-2"))

	pipelineIncludeApp1Ns := testutils.NewMetricPipelineBuilder().
		WithName("pipeline-envoy").
		WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
		WithIstioInputEnvoyMetrics(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
		Build()

	pipelineExcludeApp1Ns := testutils.NewMetricPipelineBuilder().
		WithName("pipeline-non-envoy").
		WithIstioInput(true, testutils.ExcludeNamespaces(app1Ns)).
		WithIstioInputEnvoyMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject(),
		kitk8s.NewNamespace(app2Ns, kitk8s.WithIstioInjection()).K8sObject(),
		&pipelineIncludeApp1Ns,
		&pipelineExcludeApp1Ns,
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)
	resources = append(resources, trafficgen.K8sObjects(app1Ns)...)
	resources = append(resources, trafficgen.K8sObjects(app2Ns)...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)

	Eventually(func(g Gomega) {
		backend1ExportURL := backend1.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backend1ExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(200))
		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatMetrics(
			ContainElement(HaveName(BeElementOf([]string{"envoy_cluster_version", "envoy_cluster_upstream_rq_total", "envoy_cluster_upstream_cx_total"}))),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

	Eventually(func(g Gomega) {
		backend2ExportURL := backend2.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backend2ExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(200))
		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatMetrics(
			Not(ContainElement(HaveName(BeElementOf([]string{"envoy_cluster_version", "envoy_cluster_upstream_rq_total", "envoy_cluster_upstream_cx_total"})))),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
