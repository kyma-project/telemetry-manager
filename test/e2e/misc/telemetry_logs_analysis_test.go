//go:build e2e

package misc

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/trafficgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMisc), Ordered, func() {
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
		logBackend              *kitbackend.Backend
		metricBackend           *kitbackend.Backend
		traceBackend            *kitbackend.Backend
		otelCollectorLogBackend *kitbackend.Backend
		fluentBitLogBackend     *kitbackend.Backend
		selfMonitorLogBackend   *kitbackend.Backend

		namespace       = suite.ID()
		gomegaMaxLength = format.MaxLength
		logLevelsRegexp = "ERROR|error|WARNING|warning|WARN|warn"
	)

	makeResourcesTracePipeline := func(backendName string) []client.Object {
		var objs []client.Object

		// backend
		traceBackend = kitbackend.New(namespace, kitbackend.SignalTypeTraces, kitbackend.WithName(backendName))
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
		metricBackend = kitbackend.New(namespace, kitbackend.SignalTypeMetrics, kitbackend.WithName(backendName))
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
		logBackend = kitbackend.New(namespace, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName(backendName))
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

	makeResourcesToCollectLogs := func(backendName string, containers ...string) ([]client.Object, *kitbackend.Backend) {
		var objs []client.Object

		// backends
		logBackend = kitbackend.New(namespace, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithName(backendName))
		objs = append(objs, logBackend.K8sObjects()...)

		// log pipeline
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(backendName).
			WithIncludeNamespaces(kitkyma.SystemNamespaceName).
			WithIncludeContainers(containers...).
			WithHTTPOutput(testutils.HTTPHost(logBackend.Host()), testutils.HTTPPort(logBackend.Port())).
			Build()
		objs = append(objs, &logPipeline)
		return objs, logBackend
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
			objs, otelCollectorLogBackend = makeResourcesToCollectLogs(otelCollectorLogBackendName, "collector")
			K8sObjects = append(K8sObjects, objs...)
			objs, fluentBitLogBackend = makeResourcesToCollectLogs(fluentBitLogBackendName, "fluent-bit", "exporter")
			K8sObjects = append(K8sObjects, objs...)
			objs, selfMonitorLogBackend = makeResourcesToCollectLogs(selfMonitorLogBackendName, "self-monitor")
			K8sObjects = append(K8sObjects, objs...)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(GinkgoT(), K8sObjects...)).Should(Succeed())
		})

		It("Should have running metric and trace gateways", func() {
			assert.DeploymentReady(GinkgoT(), kitkyma.MetricGatewayName)
			assert.DeploymentReady(GinkgoT(), kitkyma.TraceGatewayName)
		})

		It("Should have reachable backends", func() {
			assert.BackendReachable(GinkgoT(), logBackend)
			assert.BackendReachable(GinkgoT(), metricBackend)
			assert.BackendReachable(GinkgoT(), traceBackend)
			assert.BackendReachable(GinkgoT(), otelCollectorLogBackend)
			assert.BackendReachable(GinkgoT(), fluentBitLogBackend)
			assert.BackendReachable(GinkgoT(), selfMonitorLogBackend)
		})

		It("Should have running agents", func() {
			assert.DaemonSetReady(GinkgoT(), kitkyma.MetricAgentName)
			assert.DaemonSetReady(GinkgoT(), kitkyma.FluentBitDaemonSetName)
		})

		It("Should have running pipelines", func() {
			assert.FluentBitLogPipelineHealthy(GinkgoT(), logBackendName)
			assert.MetricPipelineHealthy(GinkgoT(), metricBackendName)
			assert.TracePipelineHealthy(GinkgoT(), traceBackendName)

			assert.FluentBitLogPipelineHealthy(GinkgoT(), otelCollectorLogBackendName)
			assert.FluentBitLogPipelineHealthy(GinkgoT(), fluentBitLogBackendName)
			assert.FluentBitLogPipelineHealthy(GinkgoT(), selfMonitorLogBackendName)
		})

		It("Should push metrics successfully", func() {
			assert.MetricsFromNamespaceDelivered(GinkgoT(), metricBackend, namespace, telemetrygen.MetricNames)
		})

		It("Should push traces successfully", func() {
			assert.TracesFromNamespaceDelivered(GinkgoT(), traceBackend, namespace)
		})

		It("Should collect logs successfully", func() {
			assert.FluentBitLogsFromPodDelivered(GinkgoT(), logBackend, "")
		})

		It("Should collect otel collector component logs successfully", func() {
			assert.FluentBitLogsFromPodDelivered(GinkgoT(), otelCollectorLogBackend, "telemetry-")
		})

		It("Should collect fluent-bit component logs successfully", func() {
			assert.FluentBitLogsFromPodDelivered(GinkgoT(), fluentBitLogBackend, "telemetry-")
		})

		It("Should collect self-monitor component logs successfully", func() {
			assert.FluentBitLogsFromPodDelivered(GinkgoT(), selfMonitorLogBackend, "telemetry-")
		})

		It("Should not have any error/warn logs in the otel collector component containers", func() {
			Consistently(func(g Gomega) {
				// TODO(skhalash): provide a way to inject custom timout into the BackendDataConsistentlyMatching helper
				queryURL := suite.ProxyClient.ProxyURLForService(otelCollectorLogBackend.Namespace(), otelCollectorLogBackend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
				resp, err := suite.ProxyClient.Get(queryURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					fluentbit.HaveFlatLogs(Not(ContainElement(SatisfyAll(
						fluentbit.HavePodName(ContainSubstring("telemetry-")),
						fluentbit.HaveLevel(MatchRegexp(logLevelsRegexp)),
						fluentbit.HaveLogBody(Not( // whitelist possible (flaky/expected) errors
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
				// TODO(skhalash): provide a way to inject custom timout into the BackendDataConsistentlyMatching helper
				queryURL := suite.ProxyClient.ProxyURLForService(fluentBitLogBackend.Namespace(), fluentBitLogBackend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
				resp, err := suite.ProxyClient.Get(queryURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					fluentbit.HaveFlatLogs(Not(ContainElement(SatisfyAll(
						fluentbit.HavePodName(ContainSubstring("telemetry-")),
						fluentbit.HaveLogBody(MatchRegexp(logLevelsRegexp)), // fluenbit does not log in JSON, so we need to check the body for errors
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not have any error/warn logs in the self-monitor containers", func() {
			Consistently(func(g Gomega) {
				// TODO(skhalash): provide a way to inject custom timout into the BackendDataConsistentlyMatching helper
				queryURL := suite.ProxyClient.ProxyURLForService(selfMonitorLogBackend.Namespace(), selfMonitorLogBackend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
				resp, err := suite.ProxyClient.Get(queryURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					fluentbit.HaveFlatLogs(Not(ContainElement(SatisfyAll(
						fluentbit.HavePodName(ContainSubstring("telemetry-")),
						fluentbit.HaveLevel(MatchRegexp(logLevelsRegexp)),
					)))),
				))
			}, consistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		AfterAll(func() {
			format.MaxLength = gomegaMaxLength // restore Gomega truncation
		})
	})
})
