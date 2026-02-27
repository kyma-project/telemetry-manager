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
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestBackpressure(t *testing.T) {
	tests := []struct {
		labelPrefix      string
		pipeline         func(includeNs string, backend *kitbackend.Backend) client.Object
		generator        func(ns string) []client.Object
		assertions       func(t *testing.T, pipelineName string)
		additionalLabels []string
	}{
		{
			labelPrefix:      suite.LabelSelfMonitorLogAgentPrefix,
			additionalLabels: []string{suite.LabelLogAgent},
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
					stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(4000)).K8sObject(),
				}
			},
			assertions: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentSomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentSomeDataDropped,
				})
			},
		},
		{
			labelPrefix:      suite.LabelSelfMonitorLogGatewayPrefix,
			additionalLabels: []string{suite.LabelLogGateway},
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
			assertions: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, pipelineName)
				assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewaySomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewaySomeDataDropped,
				})
			},
		},
		{
			labelPrefix:      suite.LabelSelfMonitorFluentBitPrefix,
			additionalLabels: []string{suite.LabelFluentBit},
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(suite.LabelSelfMonitorFluentBitPrefix).
					WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
					WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(6000)).K8sObject()}
			},
			assertions: func(t *testing.T, pipelineName string) {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, pipelineName)
				assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentBufferFillingUp, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonAgentSomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentSomeDataDropped,
				})
			},
		},
		{
			labelPrefix:      suite.LabelSelfMonitorMetricGatewayPrefix,
			additionalLabels: []string{suite.LabelMetricGateway},
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
						telemetrygen.WithRate(800),
						telemetrygen.WithWorkers(5)).
						K8sObject(),
				}
			},
			assertions: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewaySomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeMetricComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewaySomeDataDropped,
				})
			},
		},
		{
			labelPrefix:      suite.LabelSelfMonitorMetricAgentPrefix,
			additionalLabels: []string{suite.LabelMetricAgent},
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
			assertions: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
				assert.MetricPipelineHealthy(t, pipelineName)
				assert.MetricPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentSomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeMetricComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentSomeDataDropped,
				})
			},
		},
		{
			labelPrefix:      suite.LabelSelfMonitorTracesPrefix,
			additionalLabels: []string{suite.LabelTraces},
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
						telemetrygen.WithRate(800),
						telemetrygen.WithWorkers(5)).
						K8sObject(),
				}
			},
			assertions: func(t *testing.T, pipelineName string) {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, pipelineName)
				assert.TracePipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewaySomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeTraceComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewaySomeDataDropped,
				})
			},
		},
	}

	// Tests run once per test case. FIPS mode is determined by environment (FIPS_IMAGE_AVAILABLE).
	// FluentBit tests always run in no-FIPS mode via WithOverrideFIPSMode(false).
	for _, tc := range tests {
		t.Run(tc.labelPrefix, func(t *testing.T) {
			selfMonLabels, selfMonOpts := labelsForSelfMonitor(tc.labelPrefix, suite.LabelBackpressure)

			var labels []string

			labels = append(labels, suite.LabelBackpressure)
			labels = append(labels, selfMonLabels...)
			labels = append(labels, tc.additionalLabels...)

			// FluentBit doesn't support FIPS mode
			opts := selfMonOpts
			if isFluentBitTest(tc.labelPrefix) {
				opts = append(opts, kubeprep.WithOverrideFIPSMode(false))
			}

			suite.SetupTestWithOptions(t, labels, opts...)

			var (
				uniquePrefix = unique.Prefix(tc.labelPrefix)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
				backend      *kitbackend.Backend
			)

			if tc.labelPrefix == suite.LabelSelfMonitorMetricAgentPrefix {
				// Metric agent and gateway (using kyma stats receiver) both send data to backend
				// We want to simulate backpressure only on agent, so block 85% of traffic only from agent.
				backend = kitbackend.New(backendNs, signalType(tc.labelPrefix), kitbackend.WithAbortFaultInjection(85),
					kitbackend.WithDropFromSourceLabel(map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"}))
			} else {
				backend = kitbackend.New(backendNs, signalType(tc.labelPrefix), kitbackend.WithAbortFaultInjection(85))
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

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			tc.assertions(t, pipeline.GetName())
		})
	}
}
