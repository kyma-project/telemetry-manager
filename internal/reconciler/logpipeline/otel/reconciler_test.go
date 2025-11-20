package otel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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

func TestReconcile_GatewayHealthConditions(t *testing.T) {
	t.Run("log gateway probing failed", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := newTestClient(t, &pipeline)

		// Only override the gateway prober to simulate deployment fetching failure
		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(workloadstatus.ErrDeploymentFetching)

		sut := newTestReconciler(fakeClient,
			WithGatewayProber(gatewayProberStub))
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		err = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
		require.NoError(t, err)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Failed to get Deployment",
		)
	})
	t.Run("log gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := newTestClient(t, &pipeline)

		// Only override the gateway prober to simulate pod pending error
		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"})

		sut := newTestReconciler(fakeClient,
			WithGatewayProber(gatewayProberStub))
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		err = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
		require.NoError(t, err)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionFalse,
			conditions.ReasonGatewayNotReady,
			"Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
		)
	})
	t.Run("log gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := newTestClient(t, &pipeline)

		// Use all defaults - gateway is ready
		sut := newTestReconciler(fakeClient)
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeGatewayHealthy,
			metav1.ConditionTrue,
			conditions.ReasonGatewayReady,
			"Log gateway Deployment is ready",
		)
	})
}
func TestReconcile_AgentHealthConditions(t *testing.T) {
	t.Run("log agent daemonset is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
		fakeClient := newTestClient(t, &pipeline)

		// Only override the agent prober to simulate agent not ready
		agentProberStub := commonStatusStubs.NewDaemonSetProber(&workloadstatus.PodIsPendingError{Message: "Error"})

		sut := newTestReconciler(fakeClient,
			WithAgentProber(agentProberStub))
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Pod is in the pending state because container:  is not running due to: Error. Please check the container:  logs.")
	})

	t.Run("log agent daemonset is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
		fakeClient := newTestClient(t, &pipeline)

		// Use all defaults - agent is ready
		sut := newTestReconciler(fakeClient)
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonAgentReady,
			"Log agent DaemonSet is ready")
	})
}
func TestReconcile_GatewayFlowHealthy(t *testing.T) {
	t.Run("log gateway flow healthy", func(t *testing.T) {
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
				expectedMessage: "Log gateway is unable to receive logs at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=gateway-throttling",
			},
			{
				name: "some data dropped",
				probe: prober.OTelGatewayProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped in Log gateway. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.OTelGatewayProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped in Log gateway. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped",
				probe: prober.OTelGatewayProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log gateway. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelGatewayProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log gateway. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=no-logs-arrive-at-the-backend",
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
				err := sut.Reconcile(t.Context(), &pipeline)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline

				_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)
			})
		}
	})
}
func TestReconcile_AgentFlowHealthy(t *testing.T) {
	t.Run("log agent flow healthy", func(t *testing.T) {
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
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped in Log agent. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped",
				probe: prober.OTelAgentProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log agent. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelAgentProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true, SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log agent. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=no-logs-arrive-at-the-backend",
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
				err := sut.Reconcile(t.Context(), &pipeline)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline

				_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)
			})
		}
	})
}
func TestReconcile_InvalidOTTLSpecs(t *testing.T) {
	t.Run("invalid transform spec", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().Build()
		fakeClient := newTestClient(t, &pipeline)

		// Override the transform spec validator to inject error
		pipelineValidator := newTestValidator(
			withTransformSpecValidator(stubs.NewTransformSpecValidator(
				&ottl.InvalidOTTLSpecError{
					Err: fmt.Errorf("invalid TransformSpec: error while parsing statements"),
				},
			)))

		sut := newTestReconciler(fakeClient,
			WithPipelineValidator(pipelineValidator))
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonOTTLSpecInvalid,
			"Invalid TransformSpec: error while parsing statements",
		)
	})

	t.Run("invalid filter spec", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().Build()
		fakeClient := newTestClient(t, &pipeline)

		// Override the filter spec validator to inject error
		pipelineValidator := newTestValidator(
			withFilterSpecValidator(stubs.NewFilterSpecValidator(
				&ottl.InvalidOTTLSpecError{
					Err: fmt.Errorf("invalid FilterSpec: error while parsing conditions"),
				},
			)))

		sut := newTestReconciler(fakeClient,
			WithPipelineValidator(pipelineValidator))
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonOTTLSpecInvalid,
			"Invalid FilterSpec: error while parsing conditions",
		)
	})
}
func TestReconcile_AgentRequiredScenarios(t *testing.T) {
	t.Run("one log pipeline does not require an agent", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().WithApplicationInput(false).Build()
		fakeClient := newTestClient(t, &pipeline)

		// Use all defaults - agent is not required, should be deleted
		sut := newTestReconciler(fakeClient)
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonLogAgentNotRequired,
			"")
	})

	t.Run("some log pipelines do not require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithName("pipeline1").WithOTLPOutput().WithApplicationInput(false).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithName("pipeline2").WithOTLPOutput().WithApplicationInput(true).Build()
		fakeClient := newTestClient(t, &pipeline1, &pipeline2)

		// Use all defaults - handles mixed scenario
		sut := newTestReconciler(fakeClient)
		err1 := sut.Reconcile(t.Context(), &pipeline1)
		err2 := sut.Reconcile(t.Context(), &pipeline2)

		require.NoError(t, err1)
		require.NoError(t, err2)

		var updatedPipeline1 telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline1.Name}, &updatedPipeline1)

		requireHasStatusCondition(t, updatedPipeline1,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonLogAgentNotRequired,
			"")
	})
	t.Run("all log pipelines do not require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithName("pipeline1").WithOTLPOutput().WithApplicationInput(false).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithName("pipeline2").WithOTLPOutput().WithApplicationInput(false).Build()
		fakeClient := newTestClient(t, &pipeline1, &pipeline2)

		// Use all defaults - no agent required for any pipeline
		sut := newTestReconciler(fakeClient)
		err1 := sut.Reconcile(t.Context(), &pipeline1)
		err2 := sut.Reconcile(t.Context(), &pipeline2)

		require.NoError(t, err1)
		require.NoError(t, err2)

		var (
			updatedPipeline1 telemetryv1alpha1.LogPipeline
			updatedPipeline2 telemetryv1alpha1.LogPipeline
		)

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline1.Name}, &updatedPipeline1)
		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline2.Name}, &updatedPipeline2)

		requireHasStatusCondition(t, updatedPipeline1,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonLogAgentNotRequired,
			"")
		requireHasStatusCondition(t, updatedPipeline2,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonLogAgentNotRequired,
			"")
	})
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
