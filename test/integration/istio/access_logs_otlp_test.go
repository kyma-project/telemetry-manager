//go:build istio

package istio

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	"github.com/kyma-project/telemetry-manager/test/testkit/istio"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label("ttt"), Ordered, func() {
	const (
		sampleAppNs = "istio-permissive-mtls"
	)

	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
		metricPodURL     string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsOTel)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		objs = append(objs, &logPipeline)

		// Abusing metrics provider for istio access logs
		sampleApp := prommetricgen.New(sampleAppNs, prommetricgen.WithName("otlp-access-log-emitter"))
		objs = append(objs, sampleApp.Pod().K8sObject())
		metricPodURL = suite.ProxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort())

		return objs
	}

	Context("Istio", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have sample app running", func() {
			listOptions := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "metric-producer"}),
				Namespace:     sampleAppNs,
			}

			assert.PodsReady(suite.Ctx, listOptions)
		})

		It("Should have the log pipeline running", func() {
			assert.OTelLogPipelineHealthy(GinkgoT(), pipelineName)
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.LogGatewayName)
		})

		It("Should invoke the metrics endpoint to generate access logs", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(metricPodURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should verify istio OTLP access logs are present", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(log.HaveFlatLogs(HaveEach(SatisfyAll(
					log.HaveAttributes(HaveKey(BeElementOf(istio.AccessLogOTLPLogAttributeKeys))),

					log.HaveSeverityNumber(Equal(9)),
					log.HaveSeverityText(Equal("INFO")),
					log.HaveScopeName(Equal("io.kyma-project.telemetry/istio")),
					log.HaveScopeVersion(SatisfyAny(
						Equal("main"),
						MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
					)),

					log.HaveResourceAttributes(Not(SatisfyAny(
						HaveKey("cluster_name"),
						HaveKey("log_name"),
						HaveKey("zone_name"),
						HaveKey("node_name")))),
				)))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should verify istio cluster attributes are not present", func() {
			Consistently(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(log.HaveFlatLogs(HaveEach(SatisfyAll(
					Not(log.HaveResourceAttributes(HaveKey("cluster_name"))),
					Not(log.HaveResourceAttributes(HaveKey("log_name"))),
					Not(log.HaveResourceAttributes(HaveKey("zone_name"))),
					Not(log.HaveResourceAttributes(HaveKey("node_name"))),
				)))))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
