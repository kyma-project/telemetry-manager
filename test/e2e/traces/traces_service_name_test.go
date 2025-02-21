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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/servicenamebundle"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTraces), func() {
	var (
		mockNs           = ID()
		pipelineName     = ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeTraces)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(ProxyClient)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		objs = append(objs, servicenamebundle.K8sObjects(mockNs, telemetrygen.SignalTypeTraces)...)
		return objs
	}

	Context("When a TracePipeline exists", Ordered, func() {
		BeforeAll(func() {
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.TraceGatewayName)

		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.TracePipelineHealthy(Ctx, K8sClient, pipelineName)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendExportURL)
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

		It("Should set undefined service.name attribute to app.kubernetes.io/name label value", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithBothLabelsName, servicenamebundle.KubeAppLabelValue)
		})

		It("Should set undefined service.name attribute to app label value", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithAppLabelName, servicenamebundle.AppLabelValue)
		})

		It("Should set undefined service.name attribute to Deployment name", func() {
			verifyServiceNameAttr(servicenamebundle.DeploymentName, servicenamebundle.DeploymentName)
		})

		It("Should set undefined service.name attribute to StatefulSet name", func() {
			verifyServiceNameAttr(servicenamebundle.StatefulSetName, servicenamebundle.StatefulSetName)
		})

		It("Should set undefined service.name attribute to DaemonSet name", func() {
			verifyServiceNameAttr(servicenamebundle.DaemonSetName, servicenamebundle.DaemonSetName)
		})

		It("Should set undefined service.name attribute to Job name", func() {
			verifyServiceNameAttr(servicenamebundle.JobName, servicenamebundle.JobName)
		})

		It("Should set undefined service.name attribute to Pod name", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithNoLabelsName, servicenamebundle.PodWithNoLabelsName)
		})

		It("Should enrich service.name attribute when its value is unknown_service", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithUnknownServiceName, servicenamebundle.PodWithUnknownServiceName)
		})

		It("Should enrich service.name attribute when its value is following the unknown_service:<process.executable.name> pattern", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithUnknownServicePatternName, servicenamebundle.PodWithUnknownServicePatternName)
		})

		It("Should NOT enrich service.name attribute when its value is not following the unknown_service:<process.executable.name> pattern", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithInvalidStartForUnknownServicePatternName, servicenamebundle.AttrWithInvalidStartForUnknownServicePattern)
			verifyServiceNameAttr(servicenamebundle.PodWithInvalidEndForUnknownServicePatternName, servicenamebundle.AttrWithInvalidEndForUnknownServicePattern)
			verifyServiceNameAttr(servicenamebundle.PodWithMissingProcessForUnknownServicePatternName, servicenamebundle.AttrWithMissingProcessForUnknownServicePattern)
		})

		It("Should have no kyma resource attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendExportURL)
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
