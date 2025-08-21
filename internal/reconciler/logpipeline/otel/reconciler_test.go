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

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/logagent"
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
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", mock.Anything).Return(&logagent.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(workloadstatus.ErrDeploymentFetching)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
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
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"})
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
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
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
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
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&logagent.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(&workloadstatus.PodIsPendingError{Message: "Error"})

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
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
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&logagent.Config{}, nil, nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

		errToMsg := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
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
				name: "buffer filling up",
				probe: prober.OTelGatewayProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewayBufferFillingUp,
				expectedMessage: "Buffer in Log gateway nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=buffer-filling-up",
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.OTelGatewayProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonGatewayBufferFillingUp,
				expectedMessage: "Buffer in Log gateway nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=buffer-filling-up",
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
				fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

				agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
				agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&logagent.Config{}, nil, nil).Times(1)

				gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

				agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

				gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
				gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
				agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

				pipelineLock := &mocks.PipelineLock{}
				pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
				pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

				pipelineValidatorWithStubs := &Validator{
					PipelineLock:       pipelineLock,
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
				}

				errToMsg := &conditions.ErrorToMessageConverter{}
				sut := New(
					fakeClient,
					telemetryNamespace,
					moduleVersion,
					gatewayFlowHeathProber,
					agentFlowHealthProber,
					agentConfigBuilderMock,
					agentApplierDeleterMock,
					agentProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					pipelineLock,
					pipelineValidatorWithStubs,
					errToMsg)
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
				agentConfigBuilderMock.AssertExpectations(t)
				gatewayConfigBuilderMock.AssertExpectations(t)
			})
		}
	})

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
				name: "buffer filling up",
				probe: prober.OTelAgentProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentBufferFillingUp,
				expectedMessage: "Buffer in Log agent nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=buffer-filling-up",
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
				name: "some data dropped shadows other problems",
				probe: prober.OTelAgentProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					QueueAlmostFull:     true,
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
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
					QueueAlmostFull:     true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped in Log agent. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/logs?id=no-logs-arrive-at-the-backend",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).WithApplicationInput(true).Build()
				fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
				gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

				agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
				agentConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&logagent.Config{}, nil, nil).Times(1)

				gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
				gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)

				agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

				gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
				gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

				agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
				agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				pipelineLock := &mocks.PipelineLock{}
				pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
				pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

				pipelineValidatorWithStubs := &Validator{
					PipelineLock:       pipelineLock,
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
				}

				errToMsg := &conditions.ErrorToMessageConverter{}
				sut := New(
					fakeClient,
					telemetryNamespace,
					moduleVersion,
					gatewayFlowHeathProber,
					agentFlowHealthProber,
					agentConfigBuilderMock,
					agentApplierDeleterMock,
					agentProberStub,
					gatewayApplierDeleterMock,
					gatewayConfigBuilderMock,
					gatewayProberStub,
					istioStatusCheckerStub,
					pipelineLock,
					pipelineValidatorWithStubs,
					errToMsg)
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
				agentConfigBuilderMock.AssertExpectations(t)
				gatewayConfigBuilderMock.AssertExpectations(t)
			})
		}
	})

	t.Run("one log pipeline does not require an agent", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithOTLPOutput().WithApplicationInput(false).Build()
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(1)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).Return(&common.Config{}, nil, nil).Times(1)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelAgentProbeResult{}, nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			&mocks.AgentConfigBuilder{},
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
			pipelineValidatorWithStubs,
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
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline1, &pipeline2).WithStatusSubresource(&pipeline1, &pipeline2).Build()

		agentConfigBuilderMock := &mocks.AgentConfigBuilder{}
		agentConfigBuilderMock.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&logagent.Config{}, nil, nil)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelAgentProbeResult{}, nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			agentConfigBuilderMock,
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
			pipelineValidatorWithStubs,
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
		fakeClient := testutils.NewFakeClientWrapper().WithScheme(scheme).WithObjects(&pipeline1, &pipeline2).WithStatusSubresource(&pipeline1, &pipeline2).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Times(2)

		gatewayConfigBuilderMock := &mocks.GatewayConfigBuilder{}
		gatewayConfigBuilderMock.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline1, pipeline2}), mock.Anything).Return(&common.Config{}, nil, nil)

		gatewayApplierDeleterMock := &mocks.GatewayApplierDeleter{}
		gatewayApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		gatewayProberStub := commonStatusStubs.NewDeploymentSetProber(nil)
		agentProberStub := commonStatusStubs.NewDaemonSetProber(nil)

		gatewayFlowHeathProber := &mocks.GatewayFlowHealthProber{}
		gatewayFlowHeathProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelGatewayProbeResult{}, nil)

		agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
		agentFlowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelAgentProbeResult{}, nil)

		pipelineLock := &mocks.PipelineLock{}
		pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLock.On("ReleaseLock", mock.Anything).Return(nil)
		pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			PipelineLock:       pipelineLock,
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		sut := New(
			fakeClient,
			telemetryNamespace,
			moduleVersion,
			gatewayFlowHeathProber,
			agentFlowHealthProber,
			&mocks.AgentConfigBuilder{},
			agentApplierDeleterMock,
			agentProberStub,
			gatewayApplierDeleterMock,
			gatewayConfigBuilderMock,
			gatewayProberStub,
			istioStatusCheckerStub,
			pipelineLock,
			pipelineValidatorWithStubs,
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
