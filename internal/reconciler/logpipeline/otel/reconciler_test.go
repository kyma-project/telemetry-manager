package otel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	overridesHandlerStub := &logpipelinemocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", t.Context()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &stubs.IstioStatusChecker{IsActive: false}

	telemetryNamespace := "default"
	moduleVersion := "1.0.0"

	t.Run("log gateway probing failed", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(workloadstatus.ErrDeploymentFetching)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineValidatorWithStubs,
			errToMsg)
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

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("log gateway deployment is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"})
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineValidatorWithStubs,
			errToMsg)
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

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("log gateway deployment is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		pipelineValidatorWithStubs := &Validator{}

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineValidatorWithStubs,
			errToMsg)
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

		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("log agent daemonset is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().WithApplicationInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(&workloadstatus.PodIsPendingError{Message: "Error"})

		pipelineValidatorWithStubs := &Validator{}

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineValidatorWithStubs,
			errToMsg)
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Pod is in the pending state because container:  is not running due to: Error. Please check the container:  logs.")

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("log agent daemonset is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().WithApplicationInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		pipelineValidatorWithStubs := &Validator{}

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineValidatorWithStubs,
			errToMsg)
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonAgentReady,
			"Log agent DaemonSet is ready")

		agentConfigBuilderMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name            string
			probe           prober.OTelPipelineProbeResult
			probeErr        error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "prober fails",
				probeErr:        assert.AnError,
				expectedStatus:  metav1.ConditionUnknown,
				expectedReason:  conditions.ReasonSelfMonProbingFailed,
				expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing failed",
			},
			{
				name: "healthy",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonSelfMonFlowHealthy,
				expectedMessage: "No problems detected in the telemetry flow",
			},
			{
				name: "throttling",
				probe: prober.OTelPipelineProbeResult{
					Throttling: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewayThrottling,
				expectedMessage: "Log gateway is unable to receive logs at current rate.",
			},
			{
				name: "buffer filling up",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming log rate exceeds export rate.",
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming log rate exceeds export rate.",
			},
			{
				name: "some data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped.",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped.",
			},
			{
				name: "all data dropped",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped.",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.OTelPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					Throttling:          true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped.",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().WithApplicationInput(true).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

				agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
				agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&agent.Config{}, nil, nil).Times(1)

				gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

				agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				pipelineValidatorWithStubs := &Validator{}

				errToMsg := &conditions.ErrorToMessageConverter{}
				sut := New(
					fakeClient,
					telemetryNamespace,
					moduleVersion,
					flowHealthProberStub,
					agentConfigBuilderMock,
					agentApplierDeleterMock,
					agentProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					pipelineValidatorWithStubs,
					errToMsg)
				err := sut.Reconcile(context.Background(), &pipeline)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				requireHasStatusCondition(t, updatedPipeline,
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)
				agentConfigBuilderMock.AssertExpectations(t)
				gatewayConfigBuilderMock.AssertExpectations(t)
			})
		}
	})

	// TODO: "referenced secret missing" (requires SecretRefValidator to be implemented)
	// TODO: "referenced secret exists" (requires SecretRefValidator to be implemented)
	// TODO: "flow healthy" (requires SelfMonitoring to be implemented)
	// TODO: "tls conditions" (requires TLSCertValidator to be implemented)
	// TODO: "all log pipelines are non-reconcilable" (requires SecretRefValidator to be implemented)
	// TODO: "Check different Pod Error Conditions" (requires SecretRefValidator to be implemented)

	t.Run("one log pipeline does not require an agent", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().WithApplicationInput(false).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&gateway.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelPipelineProbeResult{}, nil)

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			&mocks.AgentConfigBuilder{},
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			&Validator{},
			&conditions.ErrorToMessageConverter{})
		err := sut.Reconcile(t.Context(), &pipeline)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonLogAgentNotRequired,
			"")

		agentApplierDeleterMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("some log pipelines do not require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithName("pipeline1").WithOTLPOutput().WithApplicationInput(false).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithName("pipeline2").WithOTLPOutput().WithApplicationInput(true).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline1, &pipeline2).WithStatusSubresource(&pipeline1, &pipeline2).Build()

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}), mock.Anything).Return(&agent.Config{}, nil, nil).Times(1)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}), mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelPipelineProbeResult{}, nil)

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			&Validator{},
			&conditions.ErrorToMessageConverter{})
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

		agentConfigBuilderMock.AssertExpectations(t)
		agentApplierDeleterMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
	})

	t.Run("all log pipelines do not require an agent", func(t *testing.T) {
		pipeline1 := testutils.NewLogPipelineBuilder().WithName("pipeline1").WithOTLPOutput().WithApplicationInput(false).Build()
		pipeline2 := testutils.NewLogPipelineBuilder().WithName("pipeline2").WithOTLPOutput().WithApplicationInput(false).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline1, &pipeline2).WithStatusSubresource(&pipeline1, &pipeline2).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(2)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}), mock.Anything).Return(&gateway.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &mocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelPipelineProbeResult{}, nil)

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			flowHealthProberStub,
			&mocks.AgentConfigBuilder{},
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			&Validator{},
			&conditions.ErrorToMessageConverter{})
		err1 := sut.Reconcile(t.Context(), &pipeline1)
		err2 := sut.Reconcile(t.Context(), &pipeline2)

		require.NoError(t, err1)
		require.NoError(t, err2)

		var updatedPipeline1 telemetryv1alpha1.LogPipeline

		var updatedPipeline2 telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline1.Name}, &updatedPipeline1)
		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline1.Name}, &updatedPipeline2)

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

		agentApplierDeleterMock.AssertExpectations(t)
		gatewayConfigBuilderMock.AssertExpectations(t)
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

func requireHasStatusCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func containsPipeline(p telemetryv1alpha1.LogPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.LogPipeline) bool {
		return len(pipelines) == 1 && pipelines[0].Name == p.Name
	})
}

func containsPipelines(pp []telemetryv1alpha1.LogPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.LogPipeline) bool {
		if len(pipelines) != len(pp) {
			return false
		}

		pipelineMap := make(map[string]bool)
		for _, p := range pipelines {
			pipelineMap[p.Name] = true
		}

		for _, p := range pp {
			if !pipelineMap[p.Name] {
				return false
			}
		}

		return true
	})
}
