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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelTelemetryLogsAnalysis), Ordered, func() {
	const (
		otelCollectorNs             = "tlogs-otelcollector"
		fluentBitNs                 = "tlogs-fluentbit"
		otelCollectorLogBackendName = "tlogs-otelcollector-log"
		fluentBitLogBackendName     = "tlogs-fluentbit-log"
		metricBackendName           = "tlogs-metric"
		traceBackendName            = "tlogs-trace"
		pushMetricsDepName          = "push-metrics-istiofied"
		consistentlyTimeout         = time.Second * 120
	)

	var (
		otelCollectorLogPipelineName     string
		fluentBitLogPipelineName         string
		metricPipelineName               string
		tracePipelineName                string
		otelCollectorLogbackendExportURL string
		fluentBitLogbackendExportURL     string
		metricbackendExportURL           string
		tracebackendExportURL            string
		gomegaMaxLength                  = format.MaxLength
		errorWarningLevels               = []string{
			"ERROR", "error",
			"WARNING", "warning",
			"WARN", "warn"}
	)

	makeResourcesOtelCollector := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(otelCollectorNs).K8sObject())

		// backends
		otelCollectorLogBackend := backend.New(otelCollectorNs, backend.SignalTypeLogs, backend.WithName(otelCollectorLogBackendName))
		objs = append(objs, otelCollectorLogBackend.K8sObjects()...)
		otelCollectorLogbackendExportURL = otelCollectorLogBackend.ExportURL(proxyClient)
		metricBackend := backend.New(otelCollectorNs, backend.SignalTypeMetrics, backend.WithName(metricBackendName))
		metricbackendExportURL = metricBackend.ExportURL(proxyClient)
		objs = append(objs, metricBackend.K8sObjects()...)
		traceBackend := backend.New(otelCollectorNs, backend.SignalTypeTraces, backend.WithName(traceBackendName))
		tracebackendExportURL = traceBackend.ExportURL(proxyClient)
		objs = append(objs, traceBackend.K8sObjects()...)

		// log pipeline
		otelCollectorLogPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", otelCollectorLogBackend.Name())).
			WithSecretKeyRef(otelCollectorLogBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName}).
			WithIncludeContainers([]string{"collector"})
		otelCollectorLogPipelineName = otelCollectorLogPipeline.Name()
		objs = append(objs, otelCollectorLogPipeline.K8sObject())

		// metrics & traces
		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", metricBackend.Name())).
			WithOutputEndpointFromSecret(metricBackend.HostSecretRefV1Alpha1()).
			PrometheusInput(true, kitk8s.IncludeNamespacesV1Alpha1(otelCollectorNs)).
			IstioInput(true, kitk8s.IncludeNamespacesV1Alpha1(otelCollectorNs)).
			OtlpInput(true).
			RuntimeInput(true, kitk8s.IncludeNamespacesV1Alpha1(otelCollectorNs))
		metricPipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())
		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-pipeline", traceBackend.Name())).
			WithOutputEndpointFromSecret(traceBackend.HostSecretRefV1Alpha1())
		tracePipelineName = tracePipeline.Name()
		objs = append(objs, tracePipeline.K8sObject())

		// metrics istio set-up (trafficgen & telemetrygen)
		objs = append(objs, trafficgen.K8sObjects(otelCollectorNs)...)
		objs = append(objs,
			kitk8s.NewPod("telemetrygen-metrics", otelCollectorNs).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)).K8sObject(),
			kitk8s.NewPod("telemetrygen-traces", otelCollectorNs).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeTraces)).K8sObject(),
		)

		return objs
	}

	makeResourcesFluentBit := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(fluentBitNs).K8sObject())

		// logs overrides (include agent logs)
		overrides := kitk8s.NewOverrides().WithPaused(false).WithCollectAgentLogs(true)
		objs = append(objs, overrides.K8sObject())

		// backend
		fluentBitLogBackend := backend.New(fluentBitNs, backend.SignalTypeLogs, backend.WithName(fluentBitLogBackendName))
		objs = append(objs, fluentBitLogBackend.K8sObjects()...)
		fluentBitLogbackendExportURL = fluentBitLogBackend.ExportURL(proxyClient)

		// log pipeline
		fluentBitLogPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", fluentBitLogBackend.Name())).
			WithSecretKeyRef(fluentBitLogBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithIncludeNamespaces([]string{kitkyma.SystemNamespaceName}).
			WithIncludeContainers([]string{"fluent-bit", "exporter"})
		fluentBitLogPipelineName = fluentBitLogPipeline.Name()
		objs = append(objs, fluentBitLogPipeline.K8sObject())

		return objs
	}

	Context("When OtelCollector-based components are deployed", func() {
		BeforeAll(func() {
			format.MaxLength = 0 // remove Gomega truncation
			k8sObjects := makeResourcesOtelCollector()
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: otelCollectorNs, Name: otelCollectorLogBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: otelCollectorNs, Name: metricBackendName})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: otelCollectorNs, Name: traceBackendName})
		})

		It("Should have running pipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, otelCollectorLogPipelineName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, metricPipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, tracePipelineName)
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should push metrics successfully", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, metricbackendExportURL, otelCollectorNs, telemetrygen.MetricNames)
		})

		It("Should push traces successfully", func() {
			verifiers.TracesFromNamespaceShouldBeDelivered(proxyClient, tracebackendExportURL, otelCollectorNs)
		})

		It("Should not have any ERROR/WARNING logs in the OtelCollector containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(otelCollectorLogbackendExportURL)
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

		AfterAll(func() {
			format.MaxLength = gomegaMaxLength // restore Gomega truncation
		})
	})

	Context("When FluentBit-based components are deployed", func() {
		BeforeAll(func() {
			format.MaxLength = 0 // remove Gomega truncation
			k8sObjects := makeResourcesFluentBit()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running backend", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: fluentBitNs, Name: fluentBitLogBackendName})
		})

		It("Should have a running pipeline", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, fluentBitLogPipelineName)
		})

		It("Should not have any ERROR/WARNING logs in the FluentBit containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(fluentBitLogbackendExportURL)
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
