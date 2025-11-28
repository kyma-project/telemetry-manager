package otel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestGatewayHealthCondition(t *testing.T) {
	tests := []struct {
		name              string
		gatewayProberStub *commonStatusStubs.DeploymentSetProber
		expectedCondition metav1.Condition
	}{
		{
			name: "log gateway probing failed",
			gatewayProberStub: commonStatusStubs.NewDeploymentSetProber(
				workloadstatus.ErrDeploymentFetching,
			),
			expectedCondition: metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonGatewayNotReady,
				Message: "Failed to get Deployment",
			},
		},
		{
			name: "log gateway deployment is not ready",
			gatewayProberStub: commonStatusStubs.NewDeploymentSetProber(
				&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
			),
			expectedCondition: metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonGatewayNotReady,
				Message: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
			},
		},
		{
			name:              "log gateway deployment is ready",
			gatewayProberStub: commonStatusStubs.NewDeploymentSetProber(nil),
			expectedCondition: metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonGatewayReady,
				Message: "Log gateway Deployment is ready",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
			fakeClient := newTestClient(t, &pipeline)

			sut := newTestReconciler(fakeClient,
				WithGatewayProber(tt.gatewayProberStub))
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)
			requireHasStatusConditionObject(t, result.pipeline, tt.expectedCondition)
		})
	}
}

func TestAgentHealthCondition(t *testing.T) {
	tests := []struct {
		name              string
		agentProberStub   *commonStatusStubs.DaemonSetProber
		expectedCondition metav1.Condition
	}{
		{
			name:            "log agent daomonset is not ready",
			agentProberStub: commonStatusStubs.NewDaemonSetProber(&workloadstatus.PodIsPendingError{Message: "Error"}),
			expectedCondition: metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonAgentNotReady,
				Message: "Pod is in the pending state because container:  is not running due to: Error. Please check the container:  logs.",
			},
		},
		{
			name:            "log agent daemonset is ready",
			agentProberStub: commonStatusStubs.NewDaemonSetProber(nil),
			expectedCondition: metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonAgentReady,
				Message: "Log agent DaemonSet is ready",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
			fakeClient := newTestClient(t, &pipeline)

			sut := newTestReconciler(fakeClient,
				WithAgentProber(tt.agentProberStub))
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusConditionObject(t, result.pipeline, tt.expectedCondition)
		})
	}
}
func TestGatewayFlowHealthCondition(t *testing.T) {
	tests := []struct {
		name            string
		probe           prober.OTelGatewayProbeResult
		probeErr        error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "prober fails",
			probeErr:        assert.AnError,
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  conditions.ReasonSelfMonGatewayProbingFailed,
			expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing of gateway failed",
		},
		{
			name: "healthy",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
			expectedMessage: "No problems detected in the telemetry flow",
		},
		{
			name: "throttling",
			probe: prober.OTelGatewayProbeResult{
				Throttling: true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayThrottling,
			expectedMessage: "Log gateway is unable to receive logs at current rate. See troubleshooting: " + conditions.LinkGatewayThrottling,
		},
		{
			name: "some data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped in Log gateway. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "some data dropped shadows other problems",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				Throttling:          true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped in Log gateway. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log gateway. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				Throttling:          true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log gateway. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
			fakeClient := newTestClient(t, &pipeline)

			// Only override the gateway flow health prober to inject test scenario
			gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
			gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			sut := newTestReconciler(fakeClient,
				WithGatewayFlowHealthProber(gatewayFlowHeathProber))
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)
		})
	}
}
func TestAgentFlowHealthCondition(t *testing.T) {
	tests := []struct {
		name            string
		probe           prober.OTelAgentProbeResult
		probeErr        error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "prober fails",
			probeErr:        assert.AnError,
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  conditions.ReasonSelfMonAgentProbingFailed,
			expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing of agent failed",
		},
		{
			name: "healthy",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
			expectedMessage: "No problems detected in the telemetry flow",
		},
		{
			name: "some data dropped",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentSomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped in Log agent. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log agent. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
		{
			name: "all data dropped shadows other problems",
			probe: prober.OTelAgentProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true, SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log agent. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
			fakeClient := newTestClient(t, &pipeline)

			// Only override the agent flow health prober to inject test scenario
			agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
			agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			sut := newTestReconciler(fakeClient,
				WithAgentFlowHealthProber(agentFlowHealthProber))
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			requireHasStatusCondition(t, result.pipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)
		})
	}
}
func TestOTTLSpecValidation(t *testing.T) {
	tests := []struct {
		name        string
		validator   *Validator
		condStatus  metav1.ConditionStatus
		condReason  string
		condMessage string
	}{
		{
			name: "invalid transform spec",
			validator: newTestValidator(
				WithTransformSpecValidator(stubs.NewTransformSpecValidator(
					&ottl.InvalidOTTLSpecError{
						Err: fmt.Errorf("invalid TransformSpec: error while parsing statements"),
					},
				))),
			condStatus:  metav1.ConditionFalse,
			condReason:  conditions.ReasonOTTLSpecInvalid,
			condMessage: "OTTL specification is invalid, invalid TransformSpec: error while parsing statements. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
		{
			name: "invalid filter spec",
			validator: newTestValidator(
				WithFilterSpecValidator(stubs.NewFilterSpecValidator(
					&ottl.InvalidOTTLSpecError{
						Err: fmt.Errorf("invalid FilterSpec: error while parsing conditions"),
					},
				))),
			condStatus:  metav1.ConditionFalse,
			condReason:  conditions.ReasonOTTLSpecInvalid,
			condMessage: "OTTL specification is invalid, invalid FilterSpec: error while parsing conditions. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewLogPipelineBuilder().Build()
			fakeClient := newTestClient(t, &pipeline)

			sut := newTestReconciler(fakeClient,
				WithPipelineValidator(tt.validator))
			result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
			require.NoError(t, result.err)

			cond := meta.FindStatusCondition(result.pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
			require.Equal(t, tt.condStatus, cond.Status)
			require.Equal(t, tt.condReason, cond.Reason)
			require.Equal(t, tt.condMessage, cond.Message)
		})
	}
}

func TestAgentRequiredScenarios(t *testing.T) {
	tests := []struct {
		name                         string
		pipelineConfigs              []pipelineConfig
		pipelinesToCheck             []string
		expectedConditionPerPipeline map[string]metav1.Condition
	}{
		{
			name: "one log pipeline does not require an agent",
			pipelineConfigs: []pipelineConfig{
				{name: "pipeline", applicationInput: false},
			},
			pipelinesToCheck: []string{"pipeline"},
			expectedConditionPerPipeline: map[string]metav1.Condition{
				"pipeline": {
					Type:   conditions.TypeAgentHealthy,
					Status: metav1.ConditionTrue,
					Reason: conditions.ReasonLogAgentNotRequired,
				},
			},
		},
		{
			name: "some log pipelines do not require an agent",
			pipelineConfigs: []pipelineConfig{
				{name: "pipeline1", applicationInput: false},
				{name: "pipeline2", applicationInput: true},
			},
			pipelinesToCheck: []string{"pipeline1"},
			expectedConditionPerPipeline: map[string]metav1.Condition{
				"pipeline1": {
					Type:   conditions.TypeAgentHealthy,
					Status: metav1.ConditionTrue,
					Reason: conditions.ReasonLogAgentNotRequired,
				},
				"pipeline2": {
					Type:   conditions.TypeAgentHealthy,
					Status: metav1.ConditionTrue,
					Reason: conditions.ReasonAgentReady,
				},
			},
		},
		{
			name: "all log pipelines do not require an agent",
			pipelineConfigs: []pipelineConfig{
				{name: "pipeline1", applicationInput: false},
				{name: "pipeline2", applicationInput: false},
			},
			pipelinesToCheck: []string{"pipeline1", "pipeline2"},
			expectedConditionPerPipeline: map[string]metav1.Condition{
				"pipeline1": {
					Type:   conditions.TypeAgentHealthy,
					Status: metav1.ConditionTrue,
					Reason: conditions.ReasonLogAgentNotRequired,
				},
				"pipeline2": {
					Type:   conditions.TypeAgentHealthy,
					Status: metav1.ConditionTrue,
					Reason: conditions.ReasonLogAgentNotRequired,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build pipelines from configs
			var pipelines []client.Object

			for _, cfg := range tt.pipelineConfigs {
				pipeline := testutils.NewLogPipelineBuilder().
					WithName(cfg.name).
					WithOTLPOutput().
					WithApplicationInput(cfg.applicationInput).
					Build()
				pipelines = append(pipelines, &pipeline)
			}

			fakeClient := newTestClient(t, pipelines...)
			sut := newTestReconciler(fakeClient)

			// Reconcile all pipelines
			for _, cfg := range tt.pipelineConfigs {
				result := reconcileAndGet(t, fakeClient, sut, cfg.name)
				require.NoError(t, result.err)
			}

			// Check conditions for pipelines that need verification
			for _, pipelineName := range tt.pipelinesToCheck {
				result := reconcileAndGet(t, fakeClient, sut, pipelineName)
				require.NoError(t, result.err)

				expectedCond := tt.expectedConditionPerPipeline[pipelineName]
				requireHasStatusConditionObject(t, result.pipeline, expectedCond)
			}
		})
	}
}

type pipelineConfig struct {
	name             string
	applicationInput bool
}

func TestGetPipelinesRequiringAgents(t *testing.T) {
	r := Reconciler{}

	t.Run("no pipelines", func(t *testing.T) {
		pipelines := []telemetryv1alpha1.LogPipeline{}
		require.Empty(t, r.getPipelinesRequiringAgents(pipelines))
	})

	t.Run("no pipeline requires an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithOTLPOutput().WithApplicationInput(false).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithOTLPOutput().WithApplicationInput(false).Build()
		pipelines := []telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}
		require.Empty(t, r.getPipelinesRequiringAgents(pipelines))
	})

	t.Run("some pipelines require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithOTLPOutput().WithApplicationInput(true).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithOTLPOutput().WithApplicationInput(false).Build()
		pipelines := []telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}
		require.ElementsMatch(t, []telemetryv1alpha1.LogPipeline{pipeline1}, r.getPipelinesRequiringAgents(pipelines))
	})

	t.Run("all pipelines require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithOTLPOutput().WithApplicationInput(true).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithOTLPOutput().WithApplicationInput(true).Build()
		pipelines := []telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}
		require.ElementsMatch(t, []telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}, r.getPipelinesRequiringAgents(pipelines))
	})
}
