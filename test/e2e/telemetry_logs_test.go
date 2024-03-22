//go:build e2e

package e2e

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Telemetry Error/Warning Logs", Label("wip"), Ordered, func() {
	const (
		mockNs             = "tlogs-http"
		logBackendName     = "tlogs-log-receiver"
		metricBackendName  = "tlogs-metric-receiver"
		traceBackendName   = "tlogs-trace-receiver"
		logPipelineName    = "tlogs-log-pipeline"
		metricPipelineName = "tlogs-metric-pipeline"
		tracePipelineName  = "tlogs-trace-pipeline"
	)
	var logTelemetryExportURL string
	// var metricTelemetryExportURL string
	// var traceTelemetryExportURL string
	var now time.Time

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// backends
		logBackend := backend.New(logBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, logBackend.K8sObjects()...)
		logTelemetryExportURL = logBackend.TelemetryExportURL(proxyClient)
		metricBackend := backend.New(metricBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, metricBackend.K8sObjects()...)
		// metricTelemetryExportURL = metricBackend.TelemetryExportURL(proxyClient)
		traceBackend := backend.New(traceBackendName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, traceBackend.K8sObjects()...)
		// traceTelemetryExportURL = traceBackend.TelemetryExportURL(proxyClient)

		// components
		logPipeline := kitk8s.NewLogPipeline(logPipelineName).
			WithSecretKeyRef(logBackend.HostSecretRef()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName})
		objs = append(objs, logPipeline.K8sObject())
		metricPipeline := kitk8s.NewMetricPipeline(metricPipelineName).
			WithOutputEndpointFromSecret(metricBackend.HostSecretRef())
		objs = append(objs, metricPipeline.K8sObject())
		tracePipeline := kitk8s.NewTracePipeline(tracePipelineName).
			WithOutputEndpointFromSecret(traceBackend.HostSecretRef())
		objs = append(objs, tracePipeline.K8sObject())

		return objs
	}

	Context("When telemetry components are set-up", func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			now = time.Now().UTC()
		})

		It("Should have running metric and trace gateways", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have running backends", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: logBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: metricBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: traceBackendName})
		})

		It("Should have running pipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, logPipelineName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, metricPipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, tracePipelineName)
		})

		// verifyComponentLogs := func(telemetryExportURL string) {
		// 	Consistently(func(g Gomega) {
		// 		resp, err := proxyClient.Get(telemetryExportURL)
		// 		g.Expect(err).NotTo(HaveOccurred())
		// 		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		// 		g.Expect(resp).To(HaveHTTPBody(
		// 			Not(ContainLd(ContainLogRecord(SatisfyAll(
		// 				WithPodName(ContainSubstring("telemetry-")),
		// 				WithLevel(Or(Equal("ERROR"), Equal("WARNING"))),
		// 				WithTimestamp(BeTemporally(">=", now)),
		// 			)))),
		// 		))
		// 	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		// }

		It("Should not have any ERROR/WARNING level logs in the components", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(logTelemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(Or(Equal("ERROR"), Equal("WARNING"))),
						WithTimestamp(BeTemporally(">=", now)),
					)))),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
