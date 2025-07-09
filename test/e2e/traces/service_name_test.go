//go:build e2e

package traces

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	var (
		mockNs                           = suite.ID()
		pipelineName                     = suite.ID()
		backendExportURL                 string
		podWithNoLabelsName              = "pod-with-no-labels"
		podWithUnknownServiceName        = "pod-with-unknown-service"
		podWithUnknownServicePatternName = "pod-with-unknown-service-pattern"
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeTraces)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		podSpecWithUnknownServicePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
			telemetrygen.WithServiceName("unknown_service:bash"))
		podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
			telemetrygen.WithServiceName(""))
		podSpecWithUnknownService := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
			telemetrygen.WithServiceName("unknown_service"))

		objs = append(objs,
			kitk8s.NewPod(podWithNoLabelsName, mockNs).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
			kitk8s.NewPod(podWithUnknownServicePatternName, mockNs).WithPodSpec(podSpecWithUnknownServicePattern).K8sObject(),
			kitk8s.NewPod(podWithUnknownServiceName, mockNs).WithPodSpec(podSpecWithUnknownService).K8sObject(),
		)

		return objs
	}

	Context("When a TracePipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(GinkgoT(), k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(GinkgoT(), k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(GinkgoT(), kitkyma.TraceGatewayName)

		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(GinkgoT(), types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.TracePipelineHealthy(GinkgoT(), pipelineName)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatTraces(ContainElement(SatisfyAll(
						HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
						HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
					))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		}

		It("Should set undefined service.name attribute to Pod name", func() {
			verifyServiceNameAttr(podWithNoLabelsName, podWithNoLabelsName)
		})

		It("Should enrich service.name attribute when its value is unknown_service", func() {
			verifyServiceNameAttr(podWithUnknownServiceName, podWithUnknownServiceName)
		})

		It("Should enrich service.name attribute when its value is following the unknown_service:<process.executable.name> pattern", func() {
			verifyServiceNameAttr(podWithUnknownServicePatternName, podWithUnknownServicePatternName)
		})

		It("Should have no kyma resource attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatTraces(
					Not(ContainElement(
						HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
					)),
				)))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
