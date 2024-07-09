//go:build e2e

package e2e

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTelemetryLogAnalysis), Ordered, func() {
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
		namespace                  = suite.ID()
		gomegaMaxLength            = format.MaxLength
		logLevelsRegexp            = "ERROR|error|WARNING|warning|WARN|warn"
	)

	makeResourcesTracePipeline := func(backendName string) []client.Object {
		var objs []client.Object

		//backend
		traceBackend := backend.New(namespace, backend.SignalTypeTraces, backend.WithName(backendName))
		traceBackendURL = traceBackend.ExportURL(proxyClient)
		objs = append(objs, traceBackend.K8sObjects()...)

		//pipeline
		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(backendName).
			WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		//client
		objs = append(objs, kitk8s.NewPod("telemetrygen-traces", namespace).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeTraces)).K8sObject())
		return objs
	}

	makeResourcesMetricPipeline := func(backendName string) []client.Object {
		var objs []client.Object

		//backend
		metricBackend := backend.New(namespace, backend.SignalTypeMetrics, backend.WithName(backendName))
		metricBackendURL = metricBackend.ExportURL(proxyClient)
		objs = append(objs, metricBackend.K8sObjects()...)

		//pipeline
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(backendName).
			WithPrometheusInput(true, testutils.IncludeNamespaces(namespace)).
			WithRuntimeInput(true, testutils.IncludeNamespaces(namespace)).
			WithIstioInput(true, testutils.IncludeNamespaces(namespace)).
			WithOTLPOutput(testutils.OTLPEndpoint(metricBackend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		//client
		objs = append(objs, trafficgen.K8sObjects(namespace)...)
		objs = append(objs, kitk8s.NewPod("telemetrygen-metrics", namespace).WithPodSpec(telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)).K8sObject())

		return objs
	}

	makeResourcesLogPipeline := func(backendName string) []client.Object {
		var objs []client.Object

		// backend
		logBackend := backend.New(namespace, backend.SignalTypeLogs, backend.WithName(backendName))
		logBackendURL = logBackend.ExportURL(proxyClient)
		objs = append(objs, logBackend.K8sObjects()...)

		// log pipeline
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(backendName).
			WithHTTPOutput(testutils.HTTPHost(logBackend.Host()), testutils.HTTPPort(logBackend.Port())).
			Build()
		objs = append(objs, &logPipeline)

		//no client
		return objs
	}

	makeResourcesToCollectLogs := func(backendName string, containers ...string) ([]client.Object, string) {
		var objs []client.Object

		// backends
		logBackend := backend.New(namespace, backend.SignalTypeLogs, backend.WithName(backendName))
		backendURL := logBackend.ExportURL(proxyClient)
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
			format.MaxLength = 0 // remove Gomega truncation
			var k8sObjects []client.Object
			k8sObjects = append(k8sObjects, kitk8s.NewNamespace(namespace).K8sObject())
			k8sObjects = append(k8sObjects, makeResourcesTracePipeline(traceBackendName)...)
			k8sObjects = append(k8sObjects, makeResourcesMetricPipeline(metricBackendName)...)
			k8sObjects = append(k8sObjects, makeResourcesLogPipeline(logBackendName)...)
			var objs []client.Object
			objs, otelCollectorLogBackendURL = makeResourcesToCollectLogs(otelCollectorLogBackendName, "collector")
			k8sObjects = append(k8sObjects, objs...)
			objs, fluentBitLogBackendURL = makeResourcesToCollectLogs(fluentBitLogBackendName, "fluent-bit", "exporter")
			k8sObjects = append(k8sObjects, objs...)
			objs, selfMonitorLogBackendURL = makeResourcesToCollectLogs(selfMonitorLogBackendName, "self-monitor")
			k8sObjects = append(k8sObjects, objs...)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running metric and trace gateways", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
			assert.DeploymentReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have running backends", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: logBackendName})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: metricBackendName})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: traceBackendName})

			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: otelCollectorLogBackendName})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: fluentBitLogBackendName})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: selfMonitorLogBackendName})
		})

		It("Should have running agents", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSet)
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, logBackendName)
			assert.MetricPipelineHealthy(ctx, k8sClient, metricBackendName)
			assert.TracePipelineHealthy(ctx, k8sClient, traceBackendName)

			assert.LogPipelineHealthy(ctx, k8sClient, otelCollectorLogBackendName)
			assert.LogPipelineHealthy(ctx, k8sClient, fluentBitLogBackendName)
			assert.LogPipelineHealthy(ctx, k8sClient, selfMonitorLogBackendName)
		})

		It("Should push metrics successfully", func() {
			assert.MetricsFromNamespaceDelivered(proxyClient, metricBackendURL, namespace, telemetrygen.MetricNames)
		})

		It("Should push traces successfully", func() {
			assert.TracesFromNamespaceDelivered(proxyClient, traceBackendURL, namespace)
		})

		It("Should collect logs successfully", func() {
			assert.LogsDelivered(proxyClient, "", logBackendURL)
		})

		It("Should collect otel collector component logs successfully", func() {
			assert.LogsDelivered(proxyClient, "telemetry-", otelCollectorLogBackendURL)
		})

		It("Should collect fluent-bit component logs successfully", func() {
			assert.LogsDelivered(proxyClient, "telemetry-", fluentBitLogBackendURL)
		})

		It("Should collect self-monitor component logs successfully", func() {
			assert.LogsDelivered(proxyClient, "telemetry-", selfMonitorLogBackendURL)
		})

		It("Should not have any error/warn logs in the otel collector component containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(otelCollectorLogBackendURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(MatchRegexp(logLevelsRegexp)),
						WithLogBody(Not( // whitelist possible (flaky/expected) errors
							Or(
								ContainSubstring("grpc: addrConn.createTransport failed to connect"),
								ContainSubstring("rpc error: code = Unavailable desc = no healthy upstream"),
								ContainSubstring("interrupted due to shutdown:"),
								ContainSubstring("Variable substitution using $VAR will be deprecated"),
							),
						)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any error/warn logs in the FluentBit containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(fluentBitLogBackendURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLogBody(MatchRegexp(logLevelsRegexp)), // fluenbit does not log in JSON, so we need to check the body for errors
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any error/warn logs in the self-monitor containers", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(selfMonitorLogBackendURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainLd(ContainLogRecord(SatisfyAll(
						WithPodName(ContainSubstring("telemetry-")),
						WithLevel(MatchRegexp(logLevelsRegexp)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		AfterAll(func() {
			format.MaxLength = gomegaMaxLength // restore Gomega truncation
		})
	})
})
