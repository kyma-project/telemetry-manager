//go:build e2e

package misc

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTelemetryLogAnalysis), Ordered, func() {
	const (
		consistentlyTimeout         = time.Second * 120
		traceBackendName            = "trace-backend"
		metricBackendName           = "metric-backend"
		logBackendName              = "log-backend"
		otelCollectorLogBackendName = "otel-collector-log-backend"
		fluentBitLogBackendName     = "fluent-bit-log-backend"
		selfMonitorLogBackendName   = "self-monitor-log-backend"
	)

	var (
		traceBackendURL            string
		metricBackendURL           string
		logBackendURL              string
		otelCollectorLogBackendURL string
		fluentBitLogBackendURL     string
		selfMonitorLogBackendURL   string
		namespace                  = ID()
		gomegaMaxLength            = format.MaxLength
		logLevelsRegexp            = "ERROR|error|WARNING|warning|WARN|warn"
	)

	makeResourcesTracePipeline := func(backendName string) []client.Object {
		var objs []client.Object

		// backend
		traceBackend := backend.New(namespace, backend.SignalTypeTraces, backend.WithName(backendName))
		traceBackendURL = traceBackend.ExportURL(ProxyClient)
		objs = append(objs, traceBackend.K8sObjects()...)

		// pipeline
		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(backendName).
			WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		// client
		objs = append(objs, kitk8s.NewPod("telemetrygen-traces", namespace).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeTraces)).K8sObject())
		return objs
	}

	makeResourcesMetricPipeline := func(backendName string) []client.Object {
		var objs []client.Object

		// backend
		metricBackend := backend.New(namespace, backend.SignalTypeMetrics, backend.WithName(backendName))
		metricBackendURL = metricBackend.ExportURL(ProxyClient)
		objs = append(objs, metricBackend.K8sObjects()...)

		// pipeline
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(backendName).
			WithPrometheusInput(true, testutils.IncludeNamespaces(namespace)).
			WithRuntimeInput(true, testutils.IncludeNamespaces(namespace)).
			WithIstioInput(true, testutils.IncludeNamespaces(namespace)).
			WithOTLPOutput(testutils.OTLPEndpoint(metricBackend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		// client
		objs = append(objs, trafficgen.K8sObjects(namespace)...)
		objs = append(objs, kitk8s.NewPod("telemetrygen-metrics", namespace).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)).K8sObject())

		return objs
	}

	makeResourcesLogPipeline := func(backendName string) []client.Object {
		var objs []client.Object

		// backend
		logBackend := backend.New(namespace, backend.SignalTypeLogs, backend.WithName(backendName))
		logBackendURL = logBackend.ExportURL(ProxyClient)
		objs = append(objs, logBackend.K8sObjects()...)

		// log pipeline
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(backendName).
			WithHTTPOutput(testutils.HTTPHost(logBackend.Host()), testutils.HTTPPort(logBackend.Port())).
			Build()
		objs = append(objs, &logPipeline)

		// no client
		return objs
	}

	makeResourcesToCollectLogs := func(backendName string, containers ...string) ([]client.Object, string) {
		var objs []client.Object

		// backends
		logBackend := backend.New(namespace, backend.SignalTypeLogs, backend.WithName(backendName))
		backendURL := logBackend.ExportURL(ProxyClient)
		objs = append(objs, logBackend.K8sObjects()...)

		// log pipeline
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(backendName).
			WithIncludeNamespaces(kitkyma.SystemNamespaceName).
			WithIncludeContainers(containers...).
			WithHTTPOutput(testutils.HTTPHost(logBackend.Host()), testutils.HTTPPort(logBackend.Port())).
			Build()
		objs = append(objs, &logPipeline)
		return objs, backendURL
	}

	Context("When all components are deployed", func() {
		BeforeAll(func() {
			format.MaxLength = 0 // Gomega should not truncate to have all logs in the output

			var K8sObjects []client.Object
			K8sObjects = append(K8sObjects, kitk8s.NewNamespace(namespace).K8sObject())
			K8sObjects = append(K8sObjects, makeResourcesTracePipeline(traceBackendName)...)
			K8sObjects = append(K8sObjects, makeResourcesMetricPipeline(metricBackendName)...)
			K8sObjects = append(K8sObjects, makeResourcesLogPipeline(logBackendName)...)
			var objs []client.Object
			objs, otelCollectorLogBackendURL = makeResourcesToCollectLogs(otelCollectorLogBackendName, "collector")
			K8sObjects = append(K8sObjects, objs...)
			objs, fluentBitLogBackendURL = makeResourcesToCollectLogs(fluentBitLogBackendName, "fluent-bit", "exporter")
			K8sObjects = append(K8sObjects, objs...)
			objs, selfMonitorLogBackendURL = makeResourcesToCollectLogs(selfMonitorLogBackendName, "self-monitor")
			K8sObjects = append(K8sObjects, objs...)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have running metric and trace gateways", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.MetricGatewayName)
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have running backends", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: namespace, Name: logBackendName})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: namespace, Name: metricBackendName})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: namespace, Name: traceBackendName})

			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: namespace, Name: otelCollectorLogBackendName})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: namespace, Name: fluentBitLogBackendName})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: namespace, Name: selfMonitorLogBackendName})
		})

		It("Should have running agents", func() {
			assert.DaemonSetReady(Ctx, K8sClient, kitkyma.MetricAgentName)
			assert.DaemonSetReady(Ctx, K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineHealthy(Ctx, K8sClient, logBackendName)
			assert.MetricPipelineHealthy(Ctx, K8sClient, metricBackendName)
			assert.TracePipelineHealthy(Ctx, K8sClient, traceBackendName)

			assert.LogPipelineHealthy(Ctx, K8sClient, otelCollectorLogBackendName)
			assert.LogPipelineHealthy(Ctx, K8sClient, fluentBitLogBackendName)
			assert.LogPipelineHealthy(Ctx, K8sClient, selfMonitorLogBackendName)
		})

		It("Should push metrics successfully", func() {
			assert.MetricsFromNamespaceDelivered(ProxyClient, metricBackendURL, namespace, telemetrygen.MetricNames)
		})

		It("Should push traces successfully", func() {
			assert.TracesFromNamespaceDelivered(ProxyClient, traceBackendURL, namespace)
		})

		It("Should collect logs successfully", func() {
			assert.LogsDelivered(ProxyClient, "", logBackendURL)
		})

		It("Should collect otel collector component logs successfully", func() {
			assert.LogsDelivered(ProxyClient, "telemetry-", otelCollectorLogBackendURL)
		})

		It("Should collect fluent-bit component logs successfully", func() {
			assert.LogsDelivered(ProxyClient, "telemetry-", fluentBitLogBackendURL)
		})

		It("Should collect self-monitor component logs successfully", func() {
			assert.LogsDelivered(ProxyClient, "telemetry-", selfMonitorLogBackendURL)
		})

		It("Should not have any error/warn logs in the otel collector component containers", func() {
			Consistently(func(g Gomega) {
				resp, err := ProxyClient.Get(otelCollectorLogBackendURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatFluentBitLogs(Not(ContainElement(SatisfyAll(
						HavePodName(ContainSubstring("telemetry-")),
						HaveLevel(MatchRegexp(logLevelsRegexp)),
						HaveLogBody(Not( // whitelist possible (flaky/expected) errors
							Or(
								ContainSubstring("grpc: addrConn.createTransport failed to connect"),
								ContainSubstring("rpc error: code = Unavailable desc = no healthy upstream"),
								ContainSubstring("interrupted due to shutdown:"),
							),
						)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any error/warn logs in the FluentBit containers", func() {
			Consistently(func(g Gomega) {
				resp, err := ProxyClient.Get(fluentBitLogBackendURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatFluentBitLogs(Not(ContainElement(SatisfyAll(
						HavePodName(ContainSubstring("telemetry-")),
						HaveLogBody(MatchRegexp(logLevelsRegexp)), // fluenbit does not log in JSON, so we need to check the body for errors
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any error/warn logs in the self-monitor containers", func() {
			Consistently(func(g Gomega) {
				resp, err := ProxyClient.Get(selfMonitorLogBackendURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatFluentBitLogs(Not(ContainElement(SatisfyAll(
						HavePodName(ContainSubstring("telemetry-")),
						HaveLevel(MatchRegexp(logLevelsRegexp)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		AfterAll(func() {
			format.MaxLength = gomegaMaxLength // restore Gomega truncation
		})
	})
})
