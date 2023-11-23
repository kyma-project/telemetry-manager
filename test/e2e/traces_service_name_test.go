//go:build e2e

package e2e

import (
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/servicenamebundle"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
)

var _ = Describe("Traces Service Name", Label("tracing"), func() {
	const (
		mockNs          = "trace-mocks-service-name" //#nosec G101 -- This is a false positive
		mockBackendName = "trace-receiver"
	)
	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, mockBackend.K8sObjects()...)

		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		tracePipeline := kittrace.NewPipeline("pipeline-service-name-test").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
		pipelineName = tracePipeline.Name()
		objs = append(objs, tracePipeline.K8sObject())

		objs = append(objs, servicenamebundle.K8sObjects(mockNs, telemetrygen.SignalTypeTraces)...)
		return objs
	}

	Context("When a TracePipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)

		})

		It("Should have a trace backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		verifyServiceNameAttr := func(givenPodPrefix, expectedServiceName string) {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainTd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("service.name", expectedServiceName)),
						ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
					)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		}

		It("Should set service.name to app.kubernetes.io/name label value", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithBothLabelsName, servicenamebundle.KubeAppLabelValue)
		})

		It("Should set service.name to app label value", func() {
			verifyServiceNameAttr(servicenamebundle.PodWithAppLabelName, servicenamebundle.AppLabelValue)
		})

		It("Should set service.name to Deployment name", func() {
			verifyServiceNameAttr(servicenamebundle.DeploymentName, servicenamebundle.DeploymentName)
		})

		It("Should set service.name to StatefulSet name", func() {
			verifyServiceNameAttr(servicenamebundle.StatefulSetName, servicenamebundle.StatefulSetName)
		})

		It("Should set service.name to DaemonSet name", func() {
			verifyServiceNameAttr(servicenamebundle.DaemonSetName, servicenamebundle.DaemonSetName)
		})

		It("Should set service.name to Job name", func() {
			verifyServiceNameAttr(servicenamebundle.JobName, servicenamebundle.JobName)
		})

		It("Should set service.name to unknown_service", func() {
			gatewayPushURL := proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP)
			kittraces.MakeAndSendTraces(proxyClient, gatewayPushURL)
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainTd(
						// on a Gardener cluster, API server proxy traffic is routed through vpn-shoot, so service.name is set respectively
						ContainResourceAttrs(HaveKeyWithValue("service.name", BeElementOf("unknown_service", "vpn-shoot"))),
					),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have no kyma resource attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainTd(
						ContainResourceAttrs(HaveKey(ContainSubstring("kyma"))),
					)),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
