package selfmonitor

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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestHealthy(t *testing.T) {
	tests := []struct {
		labelPrefix string
		pipeline    func(includeNs string, backend *kitbackend.Backend) client.Object
		generator   func(ns string) []client.Object
		assert      func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string)
	}{
		{
			labelPrefix: suite.LabelSelfMonitorLogAgentPrefix,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(suite.LabelSelfMonitorLogAgentPrefix).
					WithInput(testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					stdoutloggen.NewDeployment(ns).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			labelPrefix: suite.LabelSelfMonitorLogGatewayPrefix,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(suite.LabelSelfMonitorLogGatewayPrefix).
					WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			labelPrefix: suite.LabelSelfMonitorFluentBitPrefix,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(suite.LabelSelfMonitorFluentBitPrefix).
					WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
					WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					stdoutloggen.NewDeployment(ns).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, pipelineName)
				assert.FluentBitLogsFromNamespaceDelivered(t, backend, ns)
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			labelPrefix: suite.LabelSelfMonitorMetricGatewayPrefix,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(suite.LabelSelfMonitorMetricGatewayPrefix).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricsFromNamespaceDelivered(t, backend, ns, telemetrygen.MetricNames)
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			labelPrefix: suite.LabelSelfMonitorMetricAgentPrefix,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(suite.LabelSelfMonitorMetricAgentPrefix).
					WithPrometheusInput(true, testutils.IncludeNamespaces(includeNs)).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				metricProducer := prommetricgen.New(ns)

				return []client.Object{
					metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricsFromNamespaceDelivered(t, backend, ns, prommetricgen.CustomMetricNames())
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
		{
			labelPrefix: suite.LabelSelfMonitorTracesPrefix,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewTracePipelineBuilder().
					WithName(suite.LabelSelfMonitorTracesPrefix).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeTraces).K8sObject(),
				}
			},
			assert: func(t *testing.T, ns string, backend *kitbackend.Backend, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, pipelineName)
				assert.TracesFromNamespaceDelivered(t, backend, ns)
				assert.TracePipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.labelPrefix, func(t *testing.T) {
			suite.RegisterTestCase(t, label(tc.labelPrefix, suite.LabelSelfMonitorHealthySuffix))

			var (
				uniquePrefix = unique.Prefix(tc.labelPrefix)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, signalType(tc.labelPrefix))
			pipeline := tc.pipeline(genNs, backend)
			generator := tc.generator(genNs)

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, generator...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			tc.assert(t, genNs, backend, pipeline.GetName())
		})
	}
}
