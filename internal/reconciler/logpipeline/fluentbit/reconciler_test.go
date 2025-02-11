package fluentbit

import (
	"context"
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
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
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

	overridesHandlerStub := &logpipelinemocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	istioStatusCheckerStub := &stubs.IstioStatusChecker{IsActive: false}

	testConfig := fluentbit.Config{
		DaemonSet:           types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		SectionsConfigMap:   types.NamespacedName{Name: "test-telemetry-fluent-bit-sections", Namespace: "default"},
		FilesConfigMap:      types.NamespacedName{Name: "test-telemetry-fluent-bit-files", Namespace: "default"},
		LuaConfigMap:        types.NamespacedName{Name: "test-telemetry-fluent-bit-lua", Namespace: "default"},
		ParsersConfigMap:    types.NamespacedName{Name: "test-telemetry-fluent-bit-parsers", Namespace: "default"},
		EnvConfigSecret:     types.NamespacedName{Name: "test-telemetry-fluent-bit-env", Namespace: "default"},
		TLSFileConfigSecret: types.NamespacedName{Name: "test-telemetry-fluent-bit-output-tls-config", Namespace: "default"},
	}

	t.Run("should set status UnsupportedMode true if contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").WithCustomFilter("Name grep").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.True(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		require.False(t, *updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("no resources generated if app input disabled", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithApplicationInputDisabled().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)
	})

	t.Run("log agent is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(workloadstatus.ErrDaemonSetNotFound)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("DaemonSet is not yet created")

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(workloadstatus.ErrDaemonSetFetching)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &conditions.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound)),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("")

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(nil),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
		errToMsgStub.On("Convert", mock.Anything).Return("")

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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
			probe           prober.LogPipelineProbeResult
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
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus:  metav1.ConditionTrue,
				expectedReason:  conditions.ReasonSelfMonFlowHealthy,
				expectedMessage: "No problems detected in the telemetry flow",
			},
			{
				name: "buffer filling up",
				probe: prober.LogPipelineProbeResult{
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonBufferFillingUp,
				expectedMessage: "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=agent-buffer-filling-up",
			},
			{
				name: "no logs delivered",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonNoLogsDelivered,
				expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "no logs delivered shadows other problems",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
					BufferFillingUp: true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonNoLogsDelivered,
				expectedMessage: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					BufferFillingUp:     true,
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonSomeDataDropped,
				expectedMessage: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{
						AllDataDropped:  true,
						SomeDataDropped: true,
					},
				},
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonSelfMonAllDataDropped,
				expectedMessage: "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				proberStub := commonStatusStubs.NewDaemonSetProber(nil)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				pipelineValidatorWithStubs := &Validator{
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
				}

				errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
				errToMsgStub.On("Convert", mock.Anything).Return("")

				sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

				var pl1 telemetryv1alpha1.LogPipeline

				require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
				err := sut.Reconcile(context.Background(), &pl1)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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
				name:            "key parse failed",
				tlsCertErr:      tlscert.ErrKeyParseFailed,
				expectedStatus:  metav1.ConditionFalse,
				expectedReason:  conditions.ReasonTLSConfigurationInvalid,
				expectedMessage: "TLS configuration invalid: failed to parse private key",
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

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				proberStub := commonStatusStubs.NewDaemonSetProber(nil)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

				pipelineValidatorWithStubs := &Validator{
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(tt.tlsCertErr),
				}

				errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}
				errToMsgStub.On("Convert", mock.Anything).Return("")

				sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

				var pl1 telemetryv1alpha1.LogPipeline

				require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
				err := sut.Reconcile(context.Background(), &pl1)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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

				agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
				agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				proberStub := commonStatusStubs.NewDaemonSetProber(tt.probeErr)

				flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

				pipelineValidatorWithStubs := &Validator{
					EndpointValidator:  stubs.NewEndpointValidator(nil),
					TLSCertValidator:   stubs.NewTLSCertValidator(nil),
					SecretRefValidator: stubs.NewSecretRefValidator(nil),
				}

				errToMsgStub := &conditions.ErrorToMessageConverter{}

				sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

				var pl1 telemetryv1alpha1.LogPipeline

				require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
				err := sut.Reconcile(context.Background(), &pl1)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
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

		agentApplierDeleterMock := &mocks.AgentApplierDeleter{}
		agentApplierDeleterMock.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		agentApplierDeleterMock.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		flowHealthProberStub := &logpipelinemocks.FlowHealthProber{}
		flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.LogPipelineProbeResult{}, nil)

		serverErr := errors.New("failed to get secret: server error")
		pipelineValidatorWithStubs := &Validator{
			EndpointValidator:  stubs.NewEndpointValidator(nil),
			TLSCertValidator:   stubs.NewTLSCertValidator(nil),
			SecretRefValidator: stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr}),
		}

		errToMsgStub := &logpipelinemocks.ErrorToMessageConverter{}

		sut := New(fakeClient, testConfig, agentApplierDeleterMock, proberStub, flowHealthProberStub, istioStatusCheckerStub, pipelineValidatorWithStubs, errToMsgStub)

		var pl1 telemetryv1alpha1.LogPipeline

		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &pl1))
		err := sut.Reconcile(context.Background(), &pl1)
		require.True(t, errors.Is(err, serverErr))

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

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
