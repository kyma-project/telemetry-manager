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

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Traces Service Name", Label("stas"), func() {
	const (
		mockNs          = "trace-mocks-service-name"
		mockBackendName = "trace-receiver"

		kubeAppLabelValue     = "kube-workload"
		appLabelValue         = "workload"
		podWithBothLabelsName = "pod-with-both-app-labels"
		podWithAppLabelName   = "pod-with-app-label"
		deploymentName        = "deployment"
		statefulSetName       = "stateful-set"
		daemonSetName         = "daemon-set"
		jobName               = "job"
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

		objs = append(objs,
			kitk8s.NewPod(podWithBothLabelsName, mockNs).
				WithLabel("app.kubernetes.io/name", kubeAppLabelValue).
				WithLabel("app", appLabelValue).
				WithPodSpec(kitk8s.TraceGenPodSpec()).
				K8sObject(),
			kitk8s.NewPod(podWithAppLabelName, mockNs).
				WithLabel("app", appLabelValue).
				WithPodSpec(kitk8s.TraceGenPodSpec()).
				K8sObject(),
			kitk8s.NewDeployment(deploymentName, mockNs).WithPodSpec(kitk8s.TraceGenPodSpec()).K8sObject(),
			kitk8s.NewStatefulSet(statefulSetName, mockNs).WithPodSpec(kitk8s.TraceGenPodSpec()).K8sObject(),
			kitk8s.NewDaemonSet(daemonSetName, mockNs).WithPodSpec(kitk8s.TraceGenPodSpec()).K8sObject(),
			kitk8s.NewJob(jobName, mockNs).WithPodSpec(kitk8s.TraceGenPodSpec()).K8sObject(),
		)

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
					)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		}

		It("Should set service.name to app.kubernetes.io/name label value", func() {
			verifyServiceNameAttr(podWithBothLabelsName, kubeAppLabelValue)
		})

		It("Should set service.name to app label value", func() {
			verifyServiceNameAttr(podWithBothLabelsName, appLabelValue)
		})

		It("Should set service.name to Deployment name", func() {
			verifyServiceNameAttr(deploymentName, deploymentName)
		})

		It("Should set service.name to StatefulSet name", func() {
			verifyServiceNameAttr(statefulSetName, statefulSetName)
		})

		It("Should set service.name to DaemonSet name", func() {
			verifyServiceNameAttr(daemonSetName, daemonSetName)
		})

		It("Should set service.name to Job name", func() {
			verifyServiceNameAttr(jobName, jobName)
		})

		It("Should set service.name to unknown_service", func() {
			gatewayPushURL := proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP)
			_, _, _ = kittraces.MakeAndSendTraces(proxyClient, gatewayPushURL)
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainTd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("service.name", "unknown_service")),
						//WithSpans(
						//	SatisfyAll(
						//		HaveLen(len(spanIDs)),
						//		WithSpanIDs(ConsistOf(spanIDs)),
						//		HaveEach(SatisfyAll(
						//			WithTraceID(Equal(traceID)),
						//			WithSpanAttrs(BeEquivalentTo(spanAttrs.AsRaw())),
						//		)),
						//	),
						//),
					)),
				))
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
