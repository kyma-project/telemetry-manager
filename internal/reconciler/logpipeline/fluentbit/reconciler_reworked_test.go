package fluentbit

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	commonStatusMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/mocks"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestUnsupportedMode(t *testing.T) {
	tests := []struct {
		name            string
		pipelineBuilder func() telemetryv1alpha1.LogPipeline
		expectedMode    bool
	}{
		{
			name: "should set status UnsupportedMode true if contains custom plugin",
			pipelineBuilder: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					WithCustomFilter("Name grep").
					Build()
			},
			expectedMode: true,
		},
		{
			name: "should set status UnsupportedMode false if does not contains custom plugin",
			pipelineBuilder: func() telemetryv1alpha1.LogPipeline {
				return testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					Build()
			},
			expectedMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := tt.pipelineBuilder()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with mocks
			reconciler := createTestReconciler(fakeClient, pipeline)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.Equal(t, tt.expectedMode, *updatedPipeline.Status.UnsupportedMode)
		})
	}
}

func TestReferencedSecret(t *testing.T) {
	tests := []struct {
		name               string
		setupPipeline      func() (telemetryv1alpha1.LogPipeline, []client.Object)
		secretRefError     error
		expectedError      error
		expectedConditions []expectedCondition
	}{
		{
			name: "a request to the Kubernetes API server has failed when validating the secret references",
			setupPipeline: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					Build()
				return pipeline, []client.Object{&pipeline}
			},
			secretRefError: &errortypes.APIRequestFailedError{Err: errors.New("failed to get secret: server error")},
			expectedError:  errors.New("failed to get secret: server error"),
			expectedConditions: []expectedCondition{
				{
					conditionType: conditions.TypeConfigurationGenerated,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonValidationFailed,
					message:       "Pipeline validation failed due to an error from the Kubernetes API server",
				},
				{
					conditionType: conditions.TypeFlowHealthy,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonSelfMonConfigNotGenerated,
					message:       "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
				},
			},
		},
		{
			name: "referenced secret missing",
			setupPipeline: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				pipeline := testutils.NewLogPipelineBuilder().
					WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").
					Build()
				return pipeline, []client.Object{&pipeline}
			},
			secretRefError: fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound),
			expectedConditions: []expectedCondition{
				{
					conditionType: conditions.TypeConfigurationGenerated,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonReferencedSecretMissing,
					message:       "One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
				},
				{
					conditionType: conditions.TypeFlowHealthy,
					status:        metav1.ConditionFalse,
					reason:        conditions.ReasonSelfMonConfigNotGenerated,
					message:       "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
				},
			},
		},
		{
			name: "referenced secret exists",
			setupPipeline: func() (telemetryv1alpha1.LogPipeline, []client.Object) {
				secret := &corev1.Secret{
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
				return pipeline, []client.Object{&pipeline, secret}
			},
			secretRefError: nil,
			expectedConditions: []expectedCondition{
				{
					conditionType: conditions.TypeConfigurationGenerated,
					status:        metav1.ConditionTrue,
					reason:        conditions.ReasonAgentConfigured,
					message:       "LogPipeline specification is successfully applied to the configuration of Log agent",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline, objects := tt.setupPipeline()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific secret reference validation error
			reconciler := createSecretRefTestReconciler(fakeClient, pipeline, tt.secretRefError)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)

			// Check expected error
			if tt.expectedError != nil {
				require.Error(t, err)
				// For APIRequestFailedError, we need to check the wrapped error
				if apiErr, ok := tt.secretRefError.(*errortypes.APIRequestFailedError); ok {
					require.True(t, errors.Is(err, apiErr.Err))
				} else {
					require.True(t, errors.Is(err, tt.expectedError))
				}
			} else {
				require.NoError(t, err)
			}

			// Verify conditions
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

			for _, expected := range tt.expectedConditions {
				requireHasStatusCondition(t, updatedPipeline,
					expected.conditionType,
					expected.status,
					expected.reason,
					expected.message,
				)
			}
		})
	}
}

func TestLogAgent(t *testing.T) {
	tests := []struct {
		name                string
		proberError         error
		errorConverter      interface{}
		expectedCondition   metav1.ConditionStatus
		expectedReason      string
		expectedMessage     string
		setupErrorConverter func(*commonStatusMocks.ErrorToMessageConverter)
	}{
		{
			name:              "log agent is not ready",
			proberError:       workloadstatus.ErrDaemonSetNotFound,
			errorConverter:    &commonStatusMocks.ErrorToMessageConverter{},
			expectedCondition: metav1.ConditionFalse,
			expectedReason:    conditions.ReasonAgentNotReady,
			expectedMessage:   workloadstatus.ErrDaemonSetNotFound.Error(),
			setupErrorConverter: func(stub *commonStatusMocks.ErrorToMessageConverter) {
				stub.On("Convert", mock.Anything).Return("DaemonSet is not yet created")
			},
		},
		{
			name:              "log agent is ready",
			proberError:       nil,
			errorConverter:    &commonStatusMocks.ErrorToMessageConverter{},
			expectedCondition: metav1.ConditionTrue,
			expectedReason:    conditions.ReasonAgentReady,
			expectedMessage:   "Log agent DaemonSet is ready",
		},
		{
			name:              "log agent prober fails",
			proberError:       workloadstatus.ErrDaemonSetFetching,
			errorConverter:    &conditions.ErrorToMessageConverter{},
			expectedCondition: metav1.ConditionFalse,
			expectedReason:    conditions.ReasonAgentNotReady,
			expectedMessage:   "Failed to get DaemonSet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			pipeline := testutils.NewLogPipelineBuilder().WithFinalizer("FLUENT_BIT_SECTIONS_CONFIG_MAP").Build()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			// Create reconciler with specific prober error
			reconciler := createLogAgentTestReconciler(fakeClient, pipeline, tt.proberError, tt.errorConverter, tt.setupErrorConverter)

			// Execute
			var pl telemetryv1alpha1.LogPipeline
			require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &pl))
			err := reconciler.Reconcile(t.Context(), &pl)
			require.NoError(t, err)

			// Verify
			var updatedPipeline telemetryv1alpha1.LogPipeline
			_ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			requireHasStatusCondition(t, updatedPipeline,
				conditions.TypeAgentHealthy,
				tt.expectedCondition,
				tt.expectedReason,
				tt.expectedMessage,
			)
		})
	}
}

func createTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		commonStatusStubs.NewDaemonSetProber(nil),
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		&commonStatusMocks.ErrorToMessageConverter{},
	)
}

func createLogAgentTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, proberError error, errorConverter interface{}, setupErrorConverter func(*commonStatusMocks.ErrorToMessageConverter)) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(proberError)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	// Convert errorConverter to the proper type and setup if needed
	if setupErrorConverter != nil {
		if mockConverter, ok := errorConverter.(*commonStatusMocks.ErrorToMessageConverter); ok {
			setupErrorConverter(mockConverter)
		}
	}

	// Type assert to the interface expected by the New function
	var converter ErrorToMessageConverter
	switch v := errorConverter.(type) {
	case *commonStatusMocks.ErrorToMessageConverter:
		converter = v
	case *conditions.ErrorToMessageConverter:
		converter = v
	default:
		converter = &commonStatusMocks.ErrorToMessageConverter{}
	}

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		converter,
	)
}

type expectedCondition struct {
	conditionType string
	status        metav1.ConditionStatus
	reason        string
	message       string
}

func createSecretRefTestReconciler(fakeClient client.Client, pipeline telemetryv1alpha1.LogPipeline, secretRefError error) *Reconciler {
	globals := config.NewGlobal(config.WithNamespace("default"))

	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, containsPipelines([]telemetryv1alpha1.LogPipeline{pipeline}), mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	proberStub := commonStatusStubs.NewDaemonSetProber(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, pipeline.Name).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(secretRefError),
		PipelineLock:       pipelineLock,
	}

	errToMsgStub := &commonStatusMocks.ErrorToMessageConverter{}
	errToMsgStub.On("Convert", mock.Anything).Return("")

	return New(
		globals,
		fakeClient,
		agentConfigBuilder,
		agentApplierDeleter,
		proberStub,
		flowHealthProber,
		&stubs.IstioStatusChecker{IsActive: false},
		pipelineLock,
		validator,
		errToMsgStub,
	)
}
