//go:build istio

package istio

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
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
)

var _ = Describe(suite.ID(), Label(suite.LabelGardener, suite.LabelIstio), Ordered, func() {
	var (
		envoyMetricNames = []string{
			"envoy_cluster_version",
			"envoy_cluster_upstream_rq_total",
			"envoy_cluster_upstream_cx_total",
		}

		mockNs            = suite.ID()
		app1Ns            = "app-1"
		app2Ns            = "app-2"
		backend1          *kitbackend.Backend
		backend1Name      = "backend-1"
		backend1ExportURL string
		backend2          *kitbackend.Backend
		backend2Name      = "backend-2"
		backend2ExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject(),
			kitk8s.NewNamespace(app2Ns, kitk8s.WithIstioInjection()).K8sObject())

		backend1 = kitbackend.New(mockNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend1Name))
		backend1ExportURL = backend1.ExportURL(suite.ProxyClient)
		objs = append(objs, backend1.K8sObjects()...)

		pipelineIncludeApp1Ns := testutils.NewMetricPipelineBuilder().
			WithName("pipeline-envoy").
			WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
			WithIstioInputEnvoyMetrics(true).
			WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
			Build()
		objs = append(objs, &pipelineIncludeApp1Ns)

		backend2 = kitbackend.New(mockNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backend2Name))
		backend2ExportURL = backend2.ExportURL(suite.ProxyClient)
		objs = append(objs, backend2.K8sObjects()...)

		pipelineExcludeApp1Ns := testutils.NewMetricPipelineBuilder().
			WithName("pipeline-non-envoy").
			WithIstioInput(true, testutils.ExcludeNamespaces(app1Ns)).
			WithIstioInputEnvoyMetrics(false).
			WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
			Build()
		objs = append(objs, &pipelineExcludeApp1Ns)

		objs = append(objs, trafficgen.K8sObjects(app1Ns)...)
		objs = append(objs, trafficgen.K8sObjects(app2Ns)...)

		return objs
	}

	Context("When multiple metricpipelines with envoy exist", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(GinkgoT(), k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(GinkgoT(), kitkyma.MetricGatewayName)
		})

		It("Should have a running metric agent daemonset", func() {
			assert.DaemonSetReady(GinkgoT(), kitkyma.MetricAgentName)
		})

		It("Should have a metrics backend running", func() {
			assert.BackendReachable(GinkgoT(), backend1)
			assert.BackendReachable(GinkgoT(), backend2)
		})

		It("Should verify envoy metric reach backend-1", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backend1ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(HaveName(BeElementOf(envoyMetricNames))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should verify envoy metric not reach to backend-2", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backend2ExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(
					Not(ContainElement(HaveName(BeElementOf(envoyMetricNames)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
