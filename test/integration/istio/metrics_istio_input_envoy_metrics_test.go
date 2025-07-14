//go:build istio

package istio

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
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
		mockNs           = suite.ID()
		app1Ns           = suite.IDWithSuffix("app-1")
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns, kitk8s.WithIstioInjection()).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithIstioInput(true, testutils.IncludeNamespaces(app1Ns)).
			WithIstioInputEnvoyMetrics(true).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		objs = append(objs, trafficgen.K8sObjects(app1Ns)...)

		return objs
	}

	Context("App with istio-sidecar", Ordered, func() {
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
			assert.DeploymentReady(GinkgoT(), types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running metric agent daemonset", func() {
			assert.DaemonSetReady(GinkgoT(), kitkyma.MetricAgentName)
		})

		It("Should verify envoy metric scraping", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
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

	})
})
