//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Telemetry Components Error/Warning Logs Analysis", Label("telemetry-logs-analysis"), Ordered, func() {
	const (
		mockNs                      = "tlogs-http"
		otelCollectorLogBackendName = "tlogs-log-otelcollector"
		fluentBitLogBackendName     = "tlogs-log-fluentbit"
		metricBackendName           = "tlogs-metric"
		traceBackendName            = "tlogs-trace"
		pushMetricsDepName          = "push-metrics-istiofied"
		consistentlyTimeout         = time.Second * 120
	)

	var (
		otelCollectorLogPipelineName       string
		fluentBitLogPipelineName           string
		metricPipelineName                 string
		tracePipelineName                  string
		otelCollectorLogTelemetryExportURL string
		fluentBitLogTelemetryExportURL     string
		metricTelemetryExportURL           string
		traceTelemetryExportURL            string
		gomegaMaxLength                    = format.MaxLength
		errorWarningLevels                 = []string{
			"ERROR", "error",
			"WARNING", "warning",
			"WARN", "warn"}
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// logs overrides (include agent logs)
		overrides := kitk8s.NewOverrides().WithPaused(false).WithCollectAgentLogs(true)
		objs = append(objs, overrides.K8sObject())

		// backends
		otelCollectorLogBackend := backend.New(otelCollectorLogBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, otelCollectorLogBackend.K8sObjects()...)
		otelCollectorLogTelemetryExportURL = otelCollectorLogBackend.TelemetryExportURL(proxyClient)
		fluentBitLogBackend := backend.New(fluentBitLogBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, fluentBitLogBackend.K8sObjects()...)
		fluentBitLogTelemetryExportURL = fluentBitLogBackend.TelemetryExportURL(proxyClient)
		metricBackend := backend.New(metricBackendName, mockNs, backend.SignalTypeMetrics)
		metricTelemetryExportURL = metricBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, metricBackend.K8sObjects()...)
		traceBackend := backend.New(traceBackendName, mockNs, backend.SignalTypeTraces)
		traceTelemetryExportURL = traceBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, traceBackend.K8sObjects()...)

		// log pipelines
		otelCollectorLogPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", otelCollectorLogBackend.Name())).
			WithSecretKeyRef(otelCollectorLogBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName}).
			WithIncludeContainers([]string{"collector"})
		otelCollectorLogPipelineName = otelCollectorLogPipeline.Name()
		fluentBitLogPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", fluentBitLogBackend.Name())).
			WithSecretKeyRef(fluentBitLogBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName}).
			WithIncludeContainers([]string{"fluent-bit", "exporter"})
		fluentBitLogPipelineName = fluentBitLogPipeline.Name()
		objs = append(objs, otelCollectorLogPipeline.K8sObject(), fluentBitLogPipeline.K8sObject())

		// metrics & traces
		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", metricBackend.Name())).
			WithOutputEndpointFromSecret(metricBackend.HostSecretRefV1Alpha1()).
			PrometheusInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs)).
			IstioInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs)).
			OtlpInput(true).
			RuntimeInput(true, kitk8s.IncludeNamespacesV1Alpha1(mockNs))
		metricPipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())
		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-pipeline", traceBackend.Name())).
			WithOutputEndpointFromSecret(traceBackend.HostSecretRefV1Alpha1())
		tracePipelineName = tracePipeline.Name()
		objs = append(objs, tracePipeline.K8sObject())

		// metrics istio set-up (trafficgen & telemetrygen)
		objs = append(objs, trafficgen.K8sObjects(mockNs)...)
		objs = append(objs,
			kitk8s.NewPod("telemetrygen-metrics", mockNs).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)).K8sObject(),
			kitk8s.NewPod("telemetrygen-traces", mockNs).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeTraces)).K8sObject(),
		)

		return objs
	}

	Context("When telemetry components are set-up", func() {
		BeforeAll(func() {
			format.MaxLength = 0 // remove Gomega truncation
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running metric and trace gateways", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have running backends", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: otelCollectorLogBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: fluentBitLogBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: metricBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: traceBackendName})
		})

		It("Should have running pipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, otelCollectorLogPipelineName)
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, fluentBitLogPipelineName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, metricPipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, tracePipelineName)
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should push metrics successfully", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, metricTelemetryExportURL, mockNs, telemetrygen.MetricNames)
		})

		It("Should push traces successfully", func() {
			verifiers.TracesFromNamespaceShouldBeDelivered(proxyClient, traceTelemetryExportURL, mockNs)
		})

		It("Should not have any ERROR/WARNING logs in the OtelCollector containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(otelCollectorLogTelemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(BeElementOf(errorWarningLevels)),
						WithLogBody(Not( // whitelist possible (flaky/expected) errors
							Or(
								ContainSubstring("grpc: addrConn.createTransport failed to connect"),
							),
						)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any ERROR/WARNING logs in the FluentBit containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(fluentBitLogTelemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(BeElementOf(errorWarningLevels)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		AfterAll(func() {
			format.MaxLength = gomegaMaxLength // restore Gomega truncation
		})
	})
})
