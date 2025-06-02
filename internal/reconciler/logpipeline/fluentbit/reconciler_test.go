package fluentbit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
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
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	telemetryNamespace := "kyma-system"
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
			fakeClient,
			telemetryNamespace,
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

	t.Run("should set status UnsupportedMode true if contains custom plugin", func(t *testing.T) {
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
			fakeClient,
			telemetryNamespace,
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

		require.True(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
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
			fakeClient,
			telemetryNamespace,
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

		require.False(t, *updatedPipeline.Status.UnsupportedMode)
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
			fakeClient,
			telemetryNamespace,
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

	t.Run("log agent is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilder := &mocks.AgentConfigBuilder{}
		agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(workloadstatus.ErrDaemonSetNotFound)

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
		errToMsgStub.On("Convert", mock.Anything).Return("DaemonSet is not yet created")

		sut := New(
			fakeClient,
			telemetryNamespace,
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
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			workloadstatus.ErrDaemonSetNotFound.Error(),
		)
	})

	t.Run("log agent is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
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
			fakeClient,
			telemetryNamespace,
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
			conditions.TypeAgentHealthy,
			metav1.ConditionTrue,
			conditions.ReasonAgentReady,
			"Log agent DaemonSet is ready",
		)
	})

	t.Run("log agent prober fails", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilder := &mocks.AgentConfigBuilder{}
		agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(workloadstatus.ErrDaemonSetFetching)

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

		errToMsgStub := &conditions.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
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
			conditions.TypeAgentHealthy,
			metav1.ConditionFalse,
			conditions.ReasonAgentNotReady,
			"Failed to get DaemonSet",
		)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			Build()
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
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
			PipelineLock:       pipelineLockStub,
		}

		errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("")

		sut := New(
			fakeClient,
			telemetryNamespace,
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
			conditions.ReasonReferencedSecretMissing,
			"One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
		)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
		)
	})

	t.Run("referenced secret exists", func(t *testing.T) {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

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
		errToMsgStub.On("Convert", mock.Anything).Return("")

		sut := New(
			fakeClient,
			telemetryNamespace,
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
			metav1.ConditionTrue,
			conditions.ReasonAgentConfigured,
			"LogPipeline specification is successfully applied to the configuration of Log agent",
		)
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name            string
			probe           prober.FluentBitProbeResult
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
				probe: prober.FluentBitProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonSelfMonFlowHealthy,
				expectedMessage: "No problems detected in the telemetry flow",
			},
			{
				name: "buffer filling up",
				probe: prober.FluentBitProbeResult{
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=agent-buffer-filling-up",
			},
			{
				name: "no logs delivered",
				probe: prober.FluentBitProbeResult{
					NoLogsDelivered: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentNoLogsDelivered,
				expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "no logs delivered shadows other problems",
				probe: prober.FluentBitProbeResult{
					NoLogsDelivered: true,
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentNoLogsDelivered,
				expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped",
				probe: prober.FluentBitProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.FluentBitProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					BufferFillingUp:     true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped",
				probe: prober.FluentBitProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.FluentBitProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{
						AllDataDropped:  true,
						SomeDataDropped: true,
					},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAgentAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				agentConfigBuilder := &mocks.AgentConfigBuilder{}
				agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				proberStub := commonStatusStubs.NewDaemonSetProber(nil)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

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
				errToMsgStub.On("Convert", mock.Anything).Return("")

				sut := New(
					fakeClient,
					telemetryNamespace,
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
					conditions.TypeFlowHealthy,
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)
			})
		}
	})

	t.Run("tls conditions", func(t *testing.T) {
		tests := []struct {
			name                  string
			tlsCertErr            error
			expectedStatus        metav1.ConditionStatus
			expectedReason        string
			expectedMessage       string
			expectAgentConfigured bool
		}{
			{
				name:            "cert expired",
				tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateExpired,
				expectedMessage: "TLS certificate expired on 2020-11-01",
			},
			{
				name:                  "cert about to expire",
				tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
				expectedStatus:        metav1.ConditionTrue,
				expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage:       "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
				expectAgentConfigured: true,
			},
			{
				name:            "ca expired",
				tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSCertificateExpired,
				expectedMessage: "TLS CA certificate expired on 2020-11-01",
			},
			{
				name:                  "ca about to expire",
				tlsCertErr:            &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC), IsCa: true},
				expectedStatus:        metav1.ConditionTrue,
				expectedReason:        conditions.ReasonTLSCertificateAboutToExpire,
				expectedMessage:       "TLS CA certificate is about to expire, configured certificate is valid until 2024-11-01",
				expectAgentConfigured: true,
			},
			{
				name:            "cert decode failed",
				tlsCertErr:      tlscert.ErrCertDecodeFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSConfigurationInvalid,
				expectedMessage: "TLS configuration invalid: failed to decode PEM block containing certificate",
			},
			{
				name:            "key decode failed",
				tlsCertErr:      tlscert.ErrKeyDecodeFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSConfigurationInvalid,
				expectedMessage: "TLS configuration invalid: failed to decode PEM block containing private key",
			},
			{
				name:            "cert parse failed",
				tlsCertErr:      tlscert.ErrCertParseFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSConfigurationInvalid,
				expectedMessage: "TLS configuration invalid: failed to parse certificate",
			},
			{
				name:            "cert and key mismatch",
				tlsCertErr:      tlscert.ErrInvalidCertificateKeyPair,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSConfigurationInvalid,
				expectedMessage: "TLS configuration invalid: certificate and private key do not match",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithHTTPOutput(testutils.HTTPClientTLSFromString("ca", "fooCert", "fooKey")).
					Build()
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
					SecretRefValidator: stubs.NewSecretRefValidator(tt.tlsCertErr),
					PipelineLock:       pipelineLockStub,
				}

				errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}
				errToMsgStub.On("Convert", mock.Anything).Return("")

				sut := New(
					fakeClient,
					telemetryNamespace,
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
					tt.expectedStatus,
					tt.expectedReason,
					tt.expectedMessage,
				)

				if tt.expectedStatus == metav1.ConditionFalse {
					requireHasStatusCondition(t, updatedPipeline,
						conditions.TypeFlowHealthy,
						metav1.ConditionFalse,
						conditions.ReasonSelfMonConfigNotGenerated,
						"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
					)
				}
			})
		}
	})

	t.Run("Check different Pod Error Conditions", func(t *testing.T) {
		tests := []struct {
			name            string
			probeErr        error
			expectedStatus  metav1.ConditionStatus
			expectedReason  string
			expectedMessage string
		}{
			{
				name:            "pod is OOM",
				probeErr:        &workloadstatus.PodIsPendingError{ContainerName: "foo", Reason: "OOMKilled", Message: ""},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.",
			},
			{
				name:            "pod is CrashLoop",
				probeErr:        &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "Error"},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "Pod is in the pending state because container: foo is not running due to: Error. Please check the container: foo logs.",
			},
			{
				name:            "no Pods deployed",
				probeErr:        workloadstatus.ErrNoPodsDeployed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonAgentNotReady,
				expectedMessage: "No Pods deployed",
			},
			{
				name:            "fluent bit rollout in progress",
				probeErr:        &workloadstatus.RolloutInProgressError{},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonRolloutInProgress,
				expectedMessage: "Pods are being started/updated",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				agentConfigBuilder := &mocks.AgentConfigBuilder{}
				agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				proberStub := commonStatusStubs.NewDaemonSetProber(tt.probeErr)

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

				errToMsgStub := &conditions.ErrorToMessageConverter{}

				sut := New(
					fakeClient,
					telemetryNamespace,
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
				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)

				require.Equal(t, tt.expectedMessage, cond.Message)
			})
		}
	})

	t.Run("a request to the Kubernetes API server has failed when validating the secret references", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().
			WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
			Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentConfigBuilder := &mocks.AgentConfigBuilder{}
		agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil).Times(1)

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

		serverErr := errors.New("failed to get secret: server error")

		pipelineLockStub := &mocks.PipelineLock{}
		pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
		pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr}),
			PipelineLock:       pipelineLockStub,
		}

		errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}

		sut := New(
			fakeClient,
			telemetryNamespace,
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
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeConfigurationGenerated,
			metav1.ConditionFalse,
			conditions.ReasonValidationFailed,
			"Pipeline validation failed due to an error from the Kubernetes API server",
		)

		requireHasStatusCondition(t, updatedPipeline,
			conditions.TypeFlowHealthy,
			metav1.ConditionFalse,
			conditions.ReasonSelfMonConfigNotGenerated,
			"No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
		)
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
