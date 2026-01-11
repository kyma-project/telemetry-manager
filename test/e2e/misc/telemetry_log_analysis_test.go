package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	. "go.opentelemetry.io/collector/component"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTelemetryLogs(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMisc, suite.LabelFluentBit)

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
		WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.EndpointHTTP())).
		Build()

	metricBackend := kitbackend.New(metricBackendNs, kitbackend.SignalTypeMetrics)
	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(metricPipelineName).
		WithPrometheusInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithRuntimeInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithIstioInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithOTLPOutput(
			testutils.OTLPEndpoint(metricBackend.EndpointHTTP()),
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
		WithRuntimeInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(logBackend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(traceBackendNs).K8sObject(),
		kitk8sobjects.NewNamespace(metricBackendNs).K8sObject(),
		kitk8sobjects.NewNamespace(fbluentBitLogBackendNs).K8sObject(),
		kitk8sobjects.NewNamespace(logBackendNs).K8sObject(),

		kitk8sobjects.NewNamespace(genTraceNs).K8sObject(),
		kitk8sobjects.NewNamespace(genMetricNs).K8sObject(),
		kitk8sobjects.NewNamespace(genFBNs).K8sObject(),
		kitk8sobjects.NewNamespace(genLogNs).K8sObject(),

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

	assert.BackendDataConsistentlyMatches(t, logBackend, HaveFlatLogs(Not(ContainElement(SatisfyAll(
		HaveSeverityText(MatchRegexp("info|INFO|warning|WARNING")),
		HaveLogBody(ContainSubstring(StabilityLevelDeprecated.LogMessage())),
	)))),
		assert.WithOptionalDescription("log backend should not contain telemetry pod logs with deprecation info logs"))
}
