//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/servicenamebundle"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otel/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Service Name", Label("metrics"), func() {
	const (
		mockNs          = "metric-mocks-service-name"
		mockBackendName = "metric-receiver"
	)
	var (
		runtimeInputPipelineName string
		telemetryExportURL       string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)

		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		runtimeInputPipeline := kitk8s.NewMetricPipeline("pipeline-service-name-test").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			RuntimeInput(true, kitk8s.IncludeNamespaces(kitkyma.SystemNamespaceName))
		runtimeInputPipelineName = runtimeInputPipeline.Name()
		objs = append(objs, runtimeInputPipeline.K8sObject())

		objs = append(objs, servicenamebundle.K8sObjects(mockNs, telemetrygen.SignalTypeMetrics)...)

		return objs
	}

	Context("When a MetricPipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)

		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, runtimeInputPipelineName)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("service.name", expectedServiceName)),
						ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
					)),
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

		It("Should set undefined service.name attribute to unknown_service", func() {
			gatewayPushURL := proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP)
			kitmetrics.MakeAndSendGaugeMetrics(proxyClient, gatewayPushURL)
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						// on a Gardener cluster, API server proxy traffic is routed through vpn-shoot, so service.name is set respectively
						ContainResourceAttrs(HaveKeyWithValue("service.name", BeElementOf("unknown_service", "vpn-shoot"))),
					),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
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
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainMd(
						ContainResourceAttrs(HaveKey(ContainSubstring("kyma"))),
					)),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have metrics with service.name set to telemetry-metric-gateway", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("service.name", kitkyma.MetricGatewayBaseName)),
					),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have metrics with service.name set to telemetry-metric-agent", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(
						ContainResourceAttrs(HaveKeyWithValue("service.name", kitkyma.MetricAgentBaseName)),
					),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
