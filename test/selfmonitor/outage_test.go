package selfmonitor

import (
	"fmt"
	"net/http"
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

func TestOutage(t *testing.T) {
	tests := []struct {
		name        string
		component   string
		backendOpts []kitbackend.Option
		pipeline    func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object
		generator   func(ns string) []client.Object
		assertions  func(t *testing.T, pipelineName string)
	}{
		{
			name:        "log-agent",
			component:   suite.LabelLogAgent,
			backendOpts: []kitbackend.Option{kitbackend.WithAbortFaultInjection(100, http.StatusInternalServerError)},
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithInput(testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(100)).K8sObject()}
			},
			assertions: func(t *testing.T, pipelineName string) {
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
			name:        "log-gateway",
			component:   suite.LabelLogGateway,
			backendOpts: []kitbackend.Option{kitbackend.WithAbortFaultInjection(100, http.StatusInternalServerError)},
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs,
						telemetrygen.WithRate(100),
						telemetrygen.WithWorkers(1)).
						K8sObject(),
				}
			},
			assertions: func(t *testing.T, pipelineName string) {
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
			name:        "fluent-bit",
			component:   suite.LabelFluentBit,
			backendOpts: []kitbackend.Option{kitbackend.WithAbortFaultInjection(100, http.StatusBadRequest)},
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
					WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(100)).K8sObject()}
			},
			assertions: func(t *testing.T, pipelineName string) {
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
			name:        "metric-gateway",
			component:   suite.LabelMetricGateway,
			backendOpts: []kitbackend.Option{kitbackend.WithAbortFaultInjection(100, http.StatusInternalServerError)},
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(pipelineName).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics,
						telemetrygen.WithRate(100),
						telemetrygen.WithWorkers(1),
					).
						WithReplicas(2).
						K8sObject(),
				}
			},
			assertions: func(t *testing.T, pipelineName string) {
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
			name:      "metric-agent",
			component: suite.LabelMetricAgent,
			backendOpts: []kitbackend.Option{
				kitbackend.WithAbortFaultInjection(100, http.StatusInternalServerError),
				kitbackend.WithDropFromSourceLabel(map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"}),
			},
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(pipelineName).
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
			name:        "traces",
			component:   suite.LabelTraces,
			backendOpts: []kitbackend.Option{kitbackend.WithAbortFaultInjection(100, http.StatusInternalServerError)},
			pipeline: func(pipelineName, includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewTracePipelineBuilder().
					WithName(pipelineName).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
					Build()

				return &p
			},
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeTraces,
						telemetrygen.WithRate(80),
						telemetrygen.WithWorkers(1)).
						K8sObject(),
				}
			},
			assertions: func(t *testing.T, pipelineName string) {
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

	// Tests run once per test case. FIPS mode is determined by environment (FIPS_IMAGE_AVAILABLE).
	// FluentBit tests always run in no-FIPS mode via WithOverrideFIPSMode(false).
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Labels: selfmonitor + component + scenario
			labels := []string{
				suite.LabelSelfMonitor,
				tc.component,
				suite.LabelOutage,
			}

			// Outage tests need Istio for traffic simulation
			opts := []kubeprep.Option{kubeprep.WithIstio()}

			// FluentBit doesn't support FIPS mode
			if isFluentBit(tc.component) {
				opts = append(opts, kubeprep.WithOverrideFIPSMode(false))
			}

			suite.SetupTestWithOptions(t, labels, opts...)

			pipelineName := fmt.Sprintf("selfmonitor-%s", tc.name)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, signalTypeForComponent(tc.component), tc.backendOpts...)

			pipeline := tc.pipeline(pipelineName, genNs, backend)
			generator := tc.generator(genNs)

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, generator...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			// Register after CreateObjects so LIFO cleanup order ensures diagnostics run before resource deletion
			assert.SelfMonitorDebugOnFailure(t)

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			tc.assertions(t, pipeline.GetName())
		})
	}
}
