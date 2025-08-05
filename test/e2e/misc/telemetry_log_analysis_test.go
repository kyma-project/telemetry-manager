package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTelemetryLogs(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMisc)

	var (
		uniquePrefix             = unique.Prefix()
		metricPipelineName       = uniquePrefix("metric")
		tracePipelineName        = uniquePrefix("trace")
		logPipelineName          = uniquePrefix("log")
		fluentBitLogPipelineName = uniquePrefix("fluent-bit")
		traceBackendNs           = uniquePrefix("trace-backend")
		metricBackendNs          = uniquePrefix("metric-backend")
		logBackendNs             = uniquePrefix("log-backend")
		fbluentBitLogBackendNs   = uniquePrefix("fluent-bit-log-backend")
		genTraceNs               = uniquePrefix("trace-gen")
		genMetricNs              = uniquePrefix("metric-gen")
		genLogNs                 = uniquePrefix("log-gen")
		genFBNs                  = uniquePrefix("fluent-bit-gen")

		logLevelsRegexp = "ERROR|error|WARNING|warning|WARN|warn"
	)

	traceBackend := kitbackend.New(traceBackendNs, kitbackend.SignalTypeTraces)
	tracePipeline := testutils.NewTracePipelineBuilder().
		WithName(tracePipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.Endpoint())).
		Build()

	metricBackend := kitbackend.New(metricBackendNs, kitbackend.SignalTypeMetrics)
	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(metricPipelineName).
		WithPrometheusInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithRuntimeInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithIstioInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithOTLPOutput(
			testutils.OTLPEndpoint(metricBackend.Endpoint()),
		).Build()

	fluentBitLogBackend := kitbackend.New(fbluentBitLogBackendNs, kitbackend.SignalTypeLogsFluentBit)
	fluentBitLogPipeline := testutils.NewLogPipelineBuilder().
		WithName(fluentBitLogPipelineName).
		WithIncludeNamespaces(kitkyma.SystemNamespaceName).
		WithIncludeContainers("collector", "fluent-bit", "exporter", "self-monitor").
		WithHTTPOutput(testutils.HTTPHost(fluentBitLogBackend.Host()), testutils.HTTPPort(fluentBitLogBackend.Port())).
		Build()

	logBackend := kitbackend.New(logBackendNs, kitbackend.SignalTypeLogsOTel)
	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(logPipelineName).
		WithOTLPInput(true).
		WithApplicationInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(logBackend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(traceBackendNs).K8sObject(),
		kitk8s.NewNamespace(metricBackendNs).K8sObject(),
		kitk8s.NewNamespace(fbluentBitLogBackendNs).K8sObject(),
		kitk8s.NewNamespace(logBackendNs).K8sObject(),

		kitk8s.NewNamespace(genTraceNs).K8sObject(),
		kitk8s.NewNamespace(genMetricNs).K8sObject(),
		kitk8s.NewNamespace(genFBNs).K8sObject(),
		kitk8s.NewNamespace(genLogNs).K8sObject(),

		telemetrygen.NewPod(genTraceNs, telemetrygen.SignalTypeTraces).K8sObject(),
		telemetrygen.NewPod(genMetricNs, telemetrygen.SignalTypeMetrics).K8sObject(),
		telemetrygen.NewPod(genLogNs, telemetrygen.SignalTypeLogs).K8sObject(),

		&tracePipeline,
		&metricPipeline,
		&fluentBitLogPipeline,
		&logPipeline,
	}

	resources = append(resources, traceBackend.K8sObjects()...)
	resources = append(resources, metricBackend.K8sObjects()...)
	resources = append(resources, fluentBitLogBackend.K8sObjects()...)
	resources = append(resources, logBackend.K8sObjects()...)

	t.Cleanup(func() {
		if !t.Failed() {
			Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
		}
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)

	assert.BackendReachable(t, logBackend)
	assert.BackendReachable(t, metricBackend)
	assert.BackendReachable(t, traceBackend)
	assert.BackendReachable(t, fluentBitLogBackend)

	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.DaemonSetReady(t, kitkyma.LogAgentName)

	assert.FluentBitLogPipelineHealthy(t, fluentBitLogPipelineName)
	assert.TracePipelineHealthy(t, tracePipelineName)
	assert.MetricPipelineHealthy(t, metricPipelineName)
	assert.OTelLogPipelineHealthy(t, logPipelineName)

	assert.MetricsFromNamespaceDelivered(t, metricBackend, genMetricNs, telemetrygen.MetricNames)
	assert.TracesFromNamespaceDelivered(t, traceBackend, genTraceNs)
	assert.OTelLogsFromNamespaceDelivered(t, logBackend, genLogNs)
	assert.FluentBitLogsFromPodDelivered(t, fluentBitLogBackend, "telemetry-")
	assert.BackendDataConsistentlyMatches(t, fluentBitLogBackend, fluentbit.HaveFlatLogs(Not(ContainElement(SatisfyAll(
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
		assert.WithOptionalDescription("log backend should not contain telemetry pod logs with levels ERROR, WARNING or WARN"))
}
