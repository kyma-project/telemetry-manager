package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestPipelineNameClash verifies that a LogPipeline, MetricPipeline, and TracePipeline
// sharing the same name coexist correctly in the centralized OTLP Gateway.
// Before the signal-type prefix fix, same-named pipelines of different types would
// overwrite each other's OTel component entries in the shared ConfigMap.
func TestPipelineNameClash(t *testing.T) {
	suite.SetupTest(t, suite.LabelMisc, suite.LabelLogGateway, suite.LabelMetricGateway, suite.LabelTraces)

	var (
		uniquePrefix    = unique.Prefix()
		pipelineName    = uniquePrefix()
		logBackendNs    = uniquePrefix("log-backend")
		metricBackendNs = uniquePrefix("metric-backend")
		traceBackendNs  = uniquePrefix("trace-backend")
		logGenNs        = uniquePrefix("log-gen")
		metricGenNs     = uniquePrefix("metric-gen")
		traceGenNs      = uniquePrefix("trace-gen")
	)

	logBackend := kitbackend.New(logBackendNs, kitbackend.SignalTypeLogsOTel)
	metricBackend := kitbackend.New(metricBackendNs, kitbackend.SignalTypeMetrics)
	traceBackend := kitbackend.New(traceBackendNs, kitbackend.SignalTypeTraces)

	logPipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithIncludeNamespaces(logGenNs).
		WithOTLPOutput(testutils.OTLPEndpoint(logBackend.EndpointHTTP())).
		Build()

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(metricBackend.EndpointHTTP())).
		Build()

	tracePipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(logBackendNs).K8sObject(),
		kitk8sobjects.NewNamespace(metricBackendNs).K8sObject(),
		kitk8sobjects.NewNamespace(traceBackendNs).K8sObject(),
		kitk8sobjects.NewNamespace(logGenNs).K8sObject(),
		kitk8sobjects.NewNamespace(metricGenNs).K8sObject(),
		kitk8sobjects.NewNamespace(traceGenNs).K8sObject(),
		&logPipeline,
		&metricPipeline,
		&tracePipeline,
		telemetrygen.NewPod(logGenNs, telemetrygen.SignalTypeLogs).K8sObject(),
		telemetrygen.NewPod(metricGenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
		telemetrygen.NewPod(traceGenNs, telemetrygen.SignalTypeTraces).K8sObject(),
	}
	resources = append(resources, logBackend.K8sObjects()...)
	resources = append(resources, metricBackend.K8sObjects()...)
	resources = append(resources, traceBackend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, logBackend)
	assert.BackendReachable(t, metricBackend)
	assert.BackendReachable(t, traceBackend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)

	assert.OTelLogPipelineHealthy(t, pipelineName)
	assert.MetricPipelineHealthy(t, pipelineName)
	assert.TracePipelineHealthy(t, pipelineName)

	assert.OTelLogsFromNamespaceDelivered(t, logBackend, logGenNs)
	assert.MetricsFromNamespaceDelivered(t, metricBackend, metricGenNs, telemetrygen.MetricNames)
	assert.TracesFromNamespaceDelivered(t, traceBackend, traceGenNs)
}
