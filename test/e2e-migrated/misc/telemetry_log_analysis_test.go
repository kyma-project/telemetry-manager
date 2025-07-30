package misc

import (
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	"github.com/onsi/gomega/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"time"
)

func TestTelemetryLogs(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry)

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
		uniquePrefix    = unique.Prefix()
		pipelineName    = uniquePrefix()
		traceBackendNs  = uniquePrefix("trace-backend")
		metricBackendNs = uniquePrefix("metric-backend")
		genTraceNs      = uniquePrefix("trace-gen")
		genMetricNs     = uniquePrefix("metric-gen")
		genLogNs        = uniquePrefix("log-gen")
		genFBNs         = uniquePrefix("fluent-bit-gen")

		otelCollectorLogBackend *kitbackend.Backend
		selfMonitorLogBackend   *kitbackend.Backend

		namespace       = suite.ID()
		gomegaMaxLength = format.MaxLength
		logLevelsRegexp = "ERROR|error|WARNING|warning|WARN|warn"
	)

	traceBackend := kitbackend.New(traceBackendNs, kitbackend.SignalTypeTraces)
	tracePipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.Endpoint())).
		Build()

	metricBackend := kitbackend.New(metricBackendNs, kitbackend.SignalTypeMetrics)
	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithPrometheusInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithRuntimeInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithIstioInput(true, testutils.IncludeNamespaces(genMetricNs)).
		WithOTLPOutput(
			testutils.OTLPEndpoint(metricBackend.Endpoint()),
		).Build()

	fluentBitLogBackend := kitbackend.New(namespace, kitbackend.SignalTypeLogsFluentBit)
	fluentBitLogPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHost(fluentBitLogBackend.Host()), testutils.HTTPPort(fluentBitLogBackend.Port())).
		Build()
	logProducer := stdloggen.NewDeployment(genFBNs)

	logBackend := kitbackend.New(namespace, kitbackend.SignalTypeLogsOTel)
	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(logBackend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(traceBackendNs).K8sObject(),
		kitk8s.NewNamespace(genTraceNs).K8sObject(),
		kitk8s.NewNamespace(genMetricNs).K8sObject(),
		kitk8s.NewNamespace(genFBNs).K8sObject(),
		kitk8s.NewNamespace(genLogNs).K8sObject(),
		telemetrygen.NewPod(genTraceNs, telemetrygen.SignalTypeTraces).K8sObject(),
		telemetrygen.NewPod(genMetricNs, telemetrygen.SignalTypeMetrics).K8sObject(),
		telemetrygen.NewPod(genLogNs, telemetrygen.SignalTypeLogs).K8sObject(),
		logProducer.K8sObject(),
		&tracePipeline,
		&metricPipeline,
		&fluentBitLogPipeline,
		&logPipeline,
	}
	resources = append(resources, traceBackend.K8sObjects()...)
	resources = append(resources, metricBackend.K8sObjects()...)
	resources = append(resources, fluentBitLogBackend.K8sObjects()...)
	resources = append(resources, logBackend.K8sObjects()...)
}
