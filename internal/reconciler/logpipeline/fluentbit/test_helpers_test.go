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
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit/mocks"
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

// Validator test constructor and options

// validatorOption is a functional option for configuring a test Validator.
type validatorOption func(*Validator)

// withEndpointValidator overrides the default endpoint validator.
func withEndpointValidator(validator EndpointValidator) validatorOption {
	return func(v *Validator) {
		v.EndpointValidator = validator
	}
}

// withTLSCertValidator overrides the default TLS certificate validator.
func withTLSCertValidator(validator TLSCertValidator) validatorOption {
	return func(v *Validator) {
		v.TLSCertValidator = validator
	}
}

// withSecretRefValidator overrides the default secret reference validator.
func withSecretRefValidator(validator SecretRefValidator) validatorOption {
	return func(v *Validator) {
		v.SecretRefValidator = validator
	}
}

// withPipelineLock overrides the default pipeline lock.
func withPipelineLock(lock PipelineLock) validatorOption {
	return func(v *Validator) {
		v.PipelineLock = lock
	}
}

// newTestValidator creates a Validator with all dependencies mocked by default.
// Use functional options to override specific dependencies.
// All validators pass by default, and the pipeline lock succeeds by default.
func newTestValidator(opts ...validatorOption) *Validator {
	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	validator := &Validator{
		EndpointValidator:  stubs.NewEndpointValidator(nil),
		TLSCertValidator:   stubs.NewTLSCertValidator(nil),
		SecretRefValidator: stubs.NewSecretRefValidator(nil),
		PipelineLock:       pipelineLock,
	}

	// Apply functional options to override defaults
	for _, opt := range opts {
		opt(validator)
	}

	return validator
}

// Reconciler test constructor and options

// testReconcilerOption is a functional option for configuring a test Reconciler.
type testReconcilerOption func(*testReconcilerConfig)

// testReconcilerConfig holds all dependencies for creating a test reconciler.
type testReconcilerConfig struct {
	globals             config.Global
	agentConfigBuilder  AgentConfigBuilder
	agentApplierDeleter AgentApplierDeleter
	agentProber         AgentProber
	flowHealthProber    FlowHealthProber
	istioStatusChecker  IstioStatusChecker
	pipelineValidator   PipelineValidator
	errToMsgConverter   ErrorToMessageConverter
}

// withGlobals overrides the default global configuration.
func withGlobals(globals config.Global) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.globals = globals
	}
}

// withAgentConfigBuilder overrides the default agent config builder.
func withAgentConfigBuilder(builder AgentConfigBuilder) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.agentConfigBuilder = builder
	}
}

// withAgentApplierDeleter overrides the default agent applier/deleter.
func withAgentApplierDeleter(applierDeleter AgentApplierDeleter) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.agentApplierDeleter = applierDeleter
	}
}

// withAgentProber overrides the default agent prober.
func withAgentProber(prober AgentProber) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.agentProber = prober
	}
}

// withFlowHealthProber overrides the default flow health prober.
func withFlowHealthProber(prober FlowHealthProber) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.flowHealthProber = prober
	}
}

// withIstioStatusChecker overrides the default Istio status checker.
func withIstioStatusChecker(checker IstioStatusChecker) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.istioStatusChecker = checker
	}
}

// withPipelineValidator overrides the default pipeline validator.
func withPipelineValidator(validator PipelineValidator) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.pipelineValidator = validator
	}
}

// withErrorToMessageConverter overrides the default error to message converter.
func withErrorToMessageConverter(converter ErrorToMessageConverter) testReconcilerOption {
	return func(cfg *testReconcilerConfig) {
		cfg.errToMsgConverter = converter
	}
}

// newTestReconciler creates a Reconciler with all dependencies mocked by default.
// Use functional options to override specific dependencies.
//
// Default behavior:
//   - AgentConfigBuilder: Returns empty config, no errors
//   - AgentApplierDeleter: All operations succeed
//   - AgentProber: Agent is ready
//   - FlowHealthProber: Flow is healthy
//   - IstioStatusChecker: Istio is not active
//   - PipelineValidator: All validations pass
//   - ErrorToMessageConverter: Standard converter
func newTestReconciler(client client.Client, opts ...testReconcilerOption) *Reconciler {
	// Set up default mocks
	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&builder.FluentBitConfig{}, nil)

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	flowHealthProber := &logpipelinemocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.FluentBitProbeResult{}, nil)

	cfg := &testReconcilerConfig{
		globals:             config.NewGlobal(config.WithTargetNamespace("default")),
		agentConfigBuilder:  agentConfigBuilder,
		agentApplierDeleter: agentApplierDeleter,
		agentProber:         commonStatusStubs.NewDaemonSetProber(nil),
		flowHealthProber:    flowHealthProber,
		istioStatusChecker:  &stubs.IstioStatusChecker{IsActive: false},
		pipelineValidator:   newTestValidator(),
		errToMsgConverter:   &conditions.ErrorToMessageConverter{},
	}

	// Apply functional options to override defaults
	for _, opt := range opts {
		opt(cfg)
	}

	return New(
		cfg.globals,
		client,
		cfg.agentConfigBuilder,
		cfg.agentApplierDeleter,
		cfg.agentProber,
		cfg.flowHealthProber,
		cfg.istioStatusChecker,
		cfg.pipelineValidator,
		cfg.errToMsgConverter,
	)
}
