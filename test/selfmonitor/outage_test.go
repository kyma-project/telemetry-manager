package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
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

func TestOutage(t *testing.T) {
	tests := []struct {
		labelPrefix string
		pipeline    func(includeNs string, backend *kitbackend.Backend) client.Object
		generator   func(ns string) []client.Object
		assert      func(t *testing.T, pipelineName string)
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
				return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(4000)).K8sObject()}
			},
			assert: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentAllDataDropped,
				})
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
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs,
						telemetrygen.WithRate(800),
						telemetrygen.WithWorkers(5)).
						K8sObject(),
				}
			},
			assert: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
				})
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
				return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(5000)).K8sObject()}
			},
			assert: func(t *testing.T, pipelineName string) {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, pipelineName)
				assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentNoLogsDelivered, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonAgentAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentAllDataDropped,
				})
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
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics,
						telemetrygen.WithRate(10_000_000),
						telemetrygen.WithWorkers(50),
						telemetrygen.WithInterval("30s")).
						WithReplicas(2).
						K8sObject(),
				}
			},
			assert: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeMetricComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
				})
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
					metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).WithAvalancheHighLoad().K8sObject(),
					metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			assert: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeMetricComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentAllDataDropped,
				})
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
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeTraces,
						telemetrygen.WithRate(80),
						telemetrygen.WithWorkers(10)).
						K8sObject(),
				}
			},
			assert: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, pipelineName)
				assert.TracePipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeTraceComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.labelPrefix, func(t *testing.T) {
			suite.RegisterTestCase(t, label(tc.labelPrefix, suite.LabelSelfMonitorOutageSuffix))

			var (
				uniquePrefix = unique.Prefix(tc.labelPrefix)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
				backend      *kitbackend.Backend
			)
			if tc.labelPrefix == suite.LabelSelfMonitorMetricAgentPrefix {
				// Metric agent and gateway (using kyma stats receiver) both send data to backend
				// We want to simulate outage only on agent, so block all traffic only from agent.
				backend = kitbackend.New(backendNs, signalType(tc.labelPrefix), kitbackend.WithAbortFaultInjection(100),
					kitbackend.WithDropFromSourceLabel(map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"}))
			} else {
				backend = kitbackend.New(backendNs, signalType(tc.labelPrefix), kitbackend.WithReplicas(0)) // simulate outage
			}

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

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			tc.assert(t, pipeline.GetName())
		})
	}
}
