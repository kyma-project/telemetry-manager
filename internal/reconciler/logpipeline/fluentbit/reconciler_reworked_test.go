package fluentbit

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	commonStatusMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/mocks"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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
