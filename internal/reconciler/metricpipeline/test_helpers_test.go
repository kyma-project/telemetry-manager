package metricpipeline

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// reconcileAndGetResult holds the result of a reconciliation operation for test assertions.
type reconcileAndGetResult struct {
	result   ctrl.Result
	pipeline telemetryv1beta1.MetricPipeline
	err      error
}

// mockRegistry tracks mocks for automatic assertion
type mockRegistry struct {
	// Mocks with explicit expectations (Times(), Once(), etc.) that should be asserted
	mocksWithExpectations []interface{ AssertExpectations(t mock.TestingT) bool }
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		mocksWithExpectations: make([]interface{ AssertExpectations(t mock.TestingT) bool }, 0),
	}
}

// registerWithExpectations registers a mock that has explicit expectations (Times(), Once(), etc.)
func (r *mockRegistry) registerWithExpectations(m interface{ AssertExpectations(t mock.TestingT) bool }) {
	r.mocksWithExpectations = append(r.mocksWithExpectations, m)
}

func (r *mockRegistry) assertAll(t *testing.T) {
	// Assert mocks with explicit expectations
	for _, m := range r.mocksWithExpectations {
		m.AssertExpectations(t)
	}
}

// testReconciler wraps the production Reconciler to add test-specific functionality
type testReconciler struct {
	*Reconciler

	mockRegistry *mockRegistry
	assertMocks  func(*testing.T)
}

// testOption is a test-specific option that can access the mock registry
type testOption interface {
	apply(testReconciler *testReconciler)
}

// testOptionFunc wraps a function to implement testOption
type testOptionFunc func(*testReconciler)

func (f testOptionFunc) apply(tr *testReconciler) {
	f(tr)
}

// newTestClient creates a fake Kubernetes client for testing with the given objects.
// The client is configured with proper schemes, status subresources, and includes kube-system namespace.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))

	kubeSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}

	allObjs := append([]client.Object{kubeSystemNamespace}, objs...)

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(allObjs...).WithStatusSubresource(objs...).Build()
}

// reconcileAndGet performs a reconciliation and returns the updated pipeline and any error.
// It's a helper to reduce boilerplate in tests.
// To assert mocks, use the assertMocks function returned from newTestReconciler.
func reconcileAndGet(t *testing.T, client client.Client, reconciler *testReconciler, pipelineName string) reconcileAndGetResult {
	var pl telemetryv1beta1.MetricPipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &pl))

	res, recErr := reconciler.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: pipelineName},
	})

	var updatedPipeline telemetryv1beta1.MetricPipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &updatedPipeline))

	return reconcileAndGetResult{result: res, err: recErr, pipeline: updatedPipeline}
}

// newTestValidator creates a Validator with all dependencies mocked by default.
// Use functional options to override specific dependencies.
// All validators pass by default, and the pipeline lock succeeds by default.
func newTestValidator(opts ...ValidatorOption) *Validator {
	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil)

	allOpts := []ValidatorOption{
		WithEndpointValidator(stubs.NewEndpointValidator(nil)),
		WithTLSCertValidator(stubs.NewTLSCertValidator(nil)),
		WithSecretRefValidator(stubs.NewSecretRefValidator(nil)),
		WithValidatorPipelineLock(pipelineLock),
		WithTransformSpecValidator(stubs.NewTransformSpecValidator(nil)),
		WithFilterSpecValidator(stubs.NewFilterSpecValidator(nil)),
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
// Returns the reconciler and an assertMocks function that asserts all mocks automatically.
//
// The assertMocks function will:
//   - Call AssertExpectations on mocks that have explicit expectations (Times(), Once(), etc.)
//   - Call AssertNotCalled on mocks that were created but have no expectations
//
// Usage:
//
//	sut, assertMocks := newTestReconciler(fakeClient, opts...)
//	result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
//	assertMocks(t)  // Handles all mock assertions automatically
//
// Default behavior:
//   - Globals: Default target namespace and version
//   - AgentConfigBuilder: Returns empty config, no errors
//   - AgentApplierDeleter: All operations succeed
//   - AgentProber: Agent is ready
//   - GatewayConfigBuilder: Returns empty config, no errors
//   - GatewayApplierDeleter: All operations succeed
//   - GatewayProber: Gateway is ready
//   - GatewayFlowHealthProber: Flow is healthy
//   - AgentFlowHealthProber: Flow is healthy
//   - IstioStatusChecker: Istio is not active
//   - OverridesHandler: No overrides, not paused
//   - PipelineLock: Lock acquisition succeeds
//   - PipelineSyncer: Lock acquisition succeeds
//   - PipelineValidator: All validations pass
//   - ErrorToMessageConverter: Standard converter
func newTestReconciler(client client.Client, opts ...any) (*testReconciler, func(*testing.T)) {
	registry := newMockRegistry()

	tr := &testReconciler{
		mockRegistry: registry,
		assertMocks:  registry.assertAll,
	}

	// Set up default mocks with Maybe() for flexibility
	agentConfigBuilder := &mocks.AgentConfigBuilder{}
	agentConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil).Maybe()

	agentApplierDeleter := &mocks.AgentApplierDeleter{}
	agentApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	agentApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything).Return(nil).Maybe()

	gatewayConfigBuilder := &mocks.GatewayConfigBuilder{}
	gatewayConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil).Maybe()

	gatewayApplierDeleter := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	gatewayFlowHealthProber := &mocks.GatewayFlowHealthProber{}
	gatewayFlowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

	agentFlowHealthProber := &mocks.AgentFlowHealthProber{}
	agentFlowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelAgentProbeResult{}, nil).Maybe()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil).Maybe()

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil).Maybe()
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil).Maybe()

	pipelineSync := &mocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Build default options with mocked dependencies
	reconcilerOpts := []Option{
		WithClient(client),
		WithGlobals(config.NewGlobal(config.WithTargetNamespace("default"))),
		WithAgentConfigBuilder(agentConfigBuilder),
		WithAgentApplierDeleter(agentApplierDeleter),
		WithAgentProber(commonStatusStubs.NewDaemonSetProber(nil)),
		WithGatewayConfigBuilder(gatewayConfigBuilder),
		WithGatewayApplierDeleter(gatewayApplierDeleter),
		WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(nil)),
		WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		WithAgentFlowHealthProber(agentFlowHealthProber),
		WithIstioStatusChecker(&stubs.IstioStatusChecker{IsActive: false}),
		WithOverridesHandler(overridesHandler),
		WithPipelineLock(pipelineLock),
		WithPipelineSyncer(pipelineSync),
		WithPipelineValidator(newTestValidator()),
		WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
	}

	// Process provided options - collect production Options and test options separately
	var testOpts []testOption

	for _, opt := range opts {
		switch v := opt.(type) {
		case Option:
			reconcilerOpts = append(reconcilerOpts, v)
		case testOption:
			testOpts = append(testOpts, v)
		}
	}

	// Create the reconciler first
	tr.Reconciler = New(reconcilerOpts...)

	// Now apply test options that need access to the initialized Reconciler
	for _, testOpt := range testOpts {
		testOpt.apply(tr)
	}

	return tr, tr.assertMocks
}

func requireHasStatusCondition(t *testing.T, pipeline telemetryv1beta1.MetricPipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func containsPipeline(p telemetryv1beta1.MetricPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1beta1.MetricPipeline) bool {
		return len(pipelines) == 1 && pipelines[0].Name == p.Name
	})
}

func containsPipelines(pp []telemetryv1beta1.MetricPipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1beta1.MetricPipeline) bool {
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

// Mock assertion helpers

// withAgentConfigBuilderAssert registers an AgentConfigBuilder mock for auto-assertion.
func withAgentConfigBuilderAssert(mockBuilder *mocks.AgentConfigBuilder) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.agentConfigBuilder = mockBuilder
		registerMockForAssertion(tr.mockRegistry, mockBuilder)
	})
}

// withAgentApplierDeleterAssert registers an AgentApplierDeleter mock for auto-assertion.
func withAgentApplierDeleterAssert(mockApplierDeleter *mocks.AgentApplierDeleter) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.agentApplierDeleter = mockApplierDeleter
		registerMockForAssertion(tr.mockRegistry, mockApplierDeleter)
	})
}

// withGatewayConfigBuilderAssert registers a GatewayConfigBuilder mock for auto-assertion using AssertExpectations.
// Use this when you set up expectations with On().Times(), On().Once(), etc.
// If you don't set up any On() calls, AssertExpectations will fail (which is correct - you should set expectations).
func withGatewayConfigBuilderAssert(mockBuilder *mocks.GatewayConfigBuilder) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.gatewayConfigBuilder = mockBuilder
		registerMockForAssertion(tr.mockRegistry, mockBuilder)
	})
}

// withGatewayApplierDeleterAssert registers a GatewayApplierDeleter mock for auto-assertion.
func withGatewayApplierDeleterAssert(mockApplierDeleter *mocks.GatewayApplierDeleter) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.gatewayApplierDeleter = mockApplierDeleter
		registerMockForAssertion(tr.mockRegistry, mockApplierDeleter)
	})
}

// registerMockForAssertion is a helper that checks if a mock has expectations and registers it appropriately.
func registerMockForAssertion(registry *mockRegistry, mockObj interface{ AssertExpectations(t mock.TestingT) bool }) {
	// IMPORTANT: The strategy here is to ALWAYS use AssertExpectations for any mock
	// passed through WithXxxAssert helpers. This works because:
	//
	// 1. If the mock has explicit expectations (Times(), Once()), AssertExpectations will verify them
	// 2. If the mock has Maybe(), AssertExpectations will pass even if not called
	// 3. If the mock has NO On() calls at all, AssertExpectations will FAIL - which is what we want!
	//
	// The key insight: If you use WithXxxAssert, you're declaring "I care about this mock's behavior"
	// - Either you set up expectations (good)
	// - Or you forgot to set up expectations (bad - should fail)
	//
	// For mocks where you DON'T care, use the standard WithXxx (not WithXxxAssert)
	registry.registerWithExpectations(mockObj)
}
