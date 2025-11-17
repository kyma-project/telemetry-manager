package fluentbit

import (
	"testing"

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
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/mocks"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	globals := config.NewGlobal(config.WithNamespace("default"))

	overridesHandlerStub := &logpipelinemocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", t.Context()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &stubs.IstioStatusChecker{IsActive: false}

	t.Run("max pipelines exceeded", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").WithCustomFilter("Name grep").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilder := &mocks.AgentConfigBuilder{}
		agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}

		sut := New(
			globals,
			fakeClient,
			agentConfigBuilder,
			agentApplierDeleterMock,
			proberStub,
			flowHealthProberStub,
			istioStatusCheckerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsgStub,
		)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(t.Context(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline

		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonMaxPipelinesExceeded,
			"Maximum pipeline count limit exceeded",
		)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
		)
	})

	t.Run("no resources generated if app input disabled", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithApplicationInput(false).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilder := &mocks.AgentConfigBuilder{}
		agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
			PipelineLock:       pipelineLockStub,
		}

		errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}

		sut := New(
			globals,
			fakeClient,
			agentConfigBuilder,
			agentApplierDeleterMock,
			proberStub,
			flowHealthProberStub,
			istioStatusCheckerStub,
			pipelineLockStub,
			pipelineValidatorWithStubs,
			errToMsgStub,
		)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(t.Context(), &pl1)
		require.NoError(t, err)
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
