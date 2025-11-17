package fluentbit

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	commonStatusMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/mocks"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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
