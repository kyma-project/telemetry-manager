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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	logpipelinefluentbitmocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// reconcileResult holds the result of a reconciliation operation for test assertions.
type reconcileResult struct {
	pipeline telemetryv1alpha1.LogPipeline
	err      error
}

// conditionCheck defines the expected values for a status condition in tests.
type conditionCheck struct {
	condType string
	status   metav1.ConditionStatus
	reason   string
	message  string
}

// newTestClient creates a fake Kubernetes client for testing with the given objects.
// The client is configured with proper schemes and status subresources.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(objs...).Build()
}

// newTestClientWithObjs is an alias for newTestClient for backward compatibility.
// Use newTestClient directly in new tests.
func newTestClientWithObjs(t *testing.T, objs ...client.Object) client.Client {
	return newTestClient(t, objs...)
}

// reconcileAndGet performs a reconciliation and returns the updated pipeline and any error.
// It's a helper to reduce boilerplate in tests.
func reconcileAndGet(t *testing.T, client client.Client, reconciler *Reconciler, pipelineName string) reconcileResult {
	var pl telemetryv1alpha1.LogPipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &pl))

	err := reconciler.Reconcile(t.Context(), &pl)

	var updatedPipeline telemetryv1alpha1.LogPipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &updatedPipeline))

	return reconcileResult{pipeline: updatedPipeline, err: err}
}

// assertCondition verifies that a pipeline has a specific condition with expected values.
func assertCondition(t *testing.T, pipeline telemetryv1alpha1.LogPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "condition %s not found", condType)
	require.Equal(t, status, cond.Status, "condition %s has wrong status", condType)
	require.Equal(t, reason, cond.Reason, "condition %s has wrong reason", condType)
	require.Equal(t, message, cond.Message, "condition %s has wrong message", condType)
}

// newTestValidator creates a Validator with all dependencies mocked by default.
// Use functional options to override specific dependencies.
// All validators pass by default, and the pipeline lock succeeds by default.
func newTestValidator(opts ...ValidatorOption) *Validator {
	pipelineLock := &logpipelinefluentbitmocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	allOpts := []ValidatorOption{
		WithEndpointValidator(stubs.NewEndpointValidator(nil)),
		WithTLSCertValidator(stubs.NewTLSCertValidator(nil)),
		WithSecretRefValidator(stubs.NewSecretRefValidator(nil)),
		WithValidatorPipelineLock(pipelineLock),
	}

	allOpts = append(allOpts, opts...)

	validator := &Validator{}
	// Apply functional options to override defaults
	for _, opt := range allOpts {
		opt(validator)
	}

	return validator
}

// Reconciler test constructor

// newTestReconciler creates a Reconciler with all dependencies mocked by default.
// Use the production Option functions to override specific dependencies.
//
// Default behavior:
//   - AgentConfigBuilder: Returns empty config, no errors
//   - AgentApplierDeleter: All operations succeed
//   - AgentProber: Agent is ready
//   - FlowHealthProber: Flow is healthy
//   - IstioStatusChecker: Istio is not active
//   - PipelineValidator: All validations pass
//   - ErrorToMessageConverter: Standard converter
func newTestReconciler(client client.Client, opts ...Option) *Reconciler {
	// Set up default mocks
	agentConfigBuilder := &logpipelinefluentbitmocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &logpipelinefluentbitmocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.FluentBitProbeResult{}, nil)

	pipelineLock := &logpipelinefluentbitmocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	// Build default options with mocked dependencies
	allOpts := []Option{
		WithClient(client),
		WithGlobals(config.NewGlobal(config.WithTargetNamespace("default"))),
		WithAgentConfigBuilder(agentConfigBuilder),
		WithAgentApplierDeleter(agentApplierDeleter),
		WithAgentProber(commonStatusStubs.NewDaemonSetProber(nil)),
		WithFlowHealthProber(flowHealthProber),
		WithIstioStatusChecker(&stubs.IstioStatusChecker{IsActive: false}),
		WithPipelineLock(pipelineLock),
		WithPipelineValidator(newTestValidator()),
		WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
	}

	// Merge default options with provided options (provided options will override defaults)
	allOpts = append(allOpts, opts...)

	return New(allOpts...)
}
