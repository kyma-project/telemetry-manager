// Package tracepipeline provides test helpers for testing the TracePipeline reconciler.
//
// # Test Constructor Usage
//
// The newTestReconciler function creates a fully mocked reconciler for testing.
// All dependencies are mocked with sensible defaults that allow successful reconciliation.
//
// Basic usage with all defaults:
//
//	func TestMyFeature(t *testing.T) {
//	    pipeline := testutils.NewTracePipelineBuilder().Build()
//	    fakeClient := newTestClient(t, &pipeline)
//
//	    reconciler := newTestReconciler(fakeClient)
//
//	    result := reconcileAndGet(t, fakeClient, reconciler, pipeline.Name)
//	    require.NoError(t, result.err)
//	}
//
// Override specific dependencies:
//
//	func TestGatewayNotReady(t *testing.T) {
//	    pipeline := testutils.NewTracePipelineBuilder().Build()
//	    fakeClient := newTestClient(t, &pipeline)
//
//	    // Override only the gateway prober to simulate failure
//	    gatewayProber := commonStatusStubs.NewDeploymentSetProber(workloadstatus.ErrDeploymentFetching)
//	    reconciler := newTestReconciler(fakeClient,
//	        WithGatewayProber(gatewayProber),
//	    )
//
//	    result := reconcileAndGet(t, fakeClient, reconciler, pipeline.Name)
//	    require.NoError(t, result.err)
//
//	    var updatedPipeline telemetryv1alpha1.TracePipeline
//	    _ = fakeClient.Get(t.Context(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
//	    assertCondition(t, updatedPipeline, conditions.TypeGatewayHealthy,
//	        metav1.ConditionFalse, conditions.ReasonGatewayNotReady, "Failed to get Deployment")
//	}
//
// Override validator with custom behavior:
//
//	func TestSecretValidationFails(t *testing.T) {
//	    pipeline := testutils.NewTracePipelineBuilder().
//	        WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("secret", "ns", "user", "pass")).
//	        Build()
//	    fakeClient := newTestClient(t, &pipeline)
//
//	    // Create validator with custom secret validator
//	    := newTestValidator(
//	        withSecretRefValidator(stubs.NewSecretRefValidator(secretref.ErrSecretRefNotFound)),
//	    )
//	    reconciler := newTestReconciler(fakeClient,
//	        WithPipelineValidator(validator),
//	    )
//
//	    result := reconcileAndGet(t, fakeClient, reconciler, pipeline.Name)
//	    require.NoError(t, result.err)
//	}
package tracepipeline

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

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// reconcileAndGetResult holds the result of a reconciliation operation for test assertions.
type reconcileAndGetResult struct {
	result   ctrl.Result
	pipeline telemetryv1alpha1.TracePipeline
	err      error
}

// mockRegistry tracks mocks for automatic assertion
type mockRegistry struct {
	// Mocks with explicit expectations (Times(), Once(), etc.) that should be asserted
	mocksWithExpectations []interface{ AssertExpectations(t mock.TestingT) bool }
	// Mocks without expectations that should be checked for no calls
	mocksWithoutExpectations []mockWithMethods
}

type mockWithMethods struct {
	mock    interface{ AssertExpectations(t mock.TestingT) bool }
	methods []string
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		mocksWithExpectations:    make([]interface{ AssertExpectations(t mock.TestingT) bool }, 0),
		mocksWithoutExpectations: make([]mockWithMethods, 0),
	}
}

// registerWithExpectations registers a mock that has explicit expectations (Times(), Once(), etc.)
func (r *mockRegistry) registerWithExpectations(m interface{ AssertExpectations(t mock.TestingT) bool }) {
	r.mocksWithExpectations = append(r.mocksWithExpectations, m)
}

// registerWithoutExpectations registers a mock that should be checked for no calls
func (r *mockRegistry) registerWithoutExpectations(m interface{ AssertExpectations(t mock.TestingT) bool }, methods []string) {
	r.mocksWithoutExpectations = append(r.mocksWithoutExpectations, mockWithMethods{
		mock:    m,
		methods: methods,
	})
}

func (r *mockRegistry) assertAll(t *testing.T) {
	// Assert mocks with explicit expectations
	for _, m := range r.mocksWithExpectations {
		m.AssertExpectations(t)
	}

	// Assert mocks without expectations were not called
	for _, mwm := range r.mocksWithoutExpectations {
		if mockObj, ok := mwm.mock.(*mock.Mock); ok {
			for _, method := range mwm.methods {
				mockObj.AssertNotCalled(t, method)
			}
		}
	}
}

// testReconciler wraps the production Reconciler to add test-specific functionality
type testReconciler struct {
	*Reconciler
	mockRegistry *mockRegistry
	assertMocks  func(*testing.T)
}

// newTestClient creates a fake Kubernetes client for testing with the given objects.
// The client is configured with proper schemes and status subresources.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1alpha1.AddToScheme(scheme))

	kubeSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}

	allObjs := append([]client.Object{kubeSystemNamespace}, objs...)

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(allObjs...).WithStatusSubresource(objs...).Build()
}

// reconcileAndGet performs a reconciliation and returns the result and any error.
// It's a helper to reduce boilerplate in tests.
// To assert mocks, use the assertMocks function returned from newTestReconciler.
func reconcileAndGet(t *testing.T, client client.Client, reconciler *testReconciler, pipelineName string) reconcileAndGetResult {
	var pl telemetryv1alpha1.TracePipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &pl))

	res, recErr := reconciler.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: pipelineName},
	})

	var updatedPipeline telemetryv1alpha1.TracePipeline
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{Name: pipelineName}, &updatedPipeline))

	return reconcileAndGetResult{
		result:   res,
		err:      recErr,
		pipeline: updatedPipeline,
	}
}

func requireHasStatusCondition(t *testing.T, pipeline telemetryv1alpha1.TracePipeline, condType string, status metav1.ConditionStatus, reason, message string) {
	cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	require.NotNil(t, cond, "could not find condition of type %s", condType)
	require.Equal(t, status, cond.Status)
	require.Equal(t, reason, cond.Reason)
	require.Equal(t, message, cond.Message)
	require.Equal(t, pipeline.Generation, cond.ObservedGeneration)
	require.NotEmpty(t, cond.LastTransitionTime)
}

func containsPipeline(p telemetryv1alpha1.TracePipeline) any {
	return mock.MatchedBy(func(pipelines []telemetryv1alpha1.TracePipeline) bool {
		return len(pipelines) == 1 && pipelines[0].Name == p.Name
	})
}

// Validator test constructor and options

// validatorOption is a functional option for configuring a test Validator.
type validatorOption func(*Validator)

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

// withTransformSpecValidator overrides the default transform spec validator.
func withTransformSpecValidator(validator TransformSpecValidator) validatorOption {
	return func(v *Validator) {
		v.TransformSpecValidator = validator
	}
}

// withFilterSpecValidator overrides the default filter spec validator.
func withFilterSpecValidator(validator FilterSpecValidator) validatorOption {
	return func(v *Validator) {
		v.FilterSpecValidator = validator
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
		EndpointValidator:      stubs.NewEndpointValidator(nil),
		TLSCertValidator:       stubs.NewTLSCertValidator(nil),
		SecretRefValidator:     stubs.NewSecretRefValidator(nil),
		PipelineLock:           pipelineLock,
		TransformSpecValidator: stubs.NewTransformSpecValidator(nil),
		FilterSpecValidator:    stubs.NewFilterSpecValidator(nil),
	}

	// Apply functional options to override defaults
	for _, opt := range opts {
		opt(validator)
	}

	return validator
}

// Reconciler test constructor

// testOption is a test-specific option that can access the mock registry
type testOption interface {
	apply(*testReconciler)
}

// testOptionFunc wraps a function to implement testOption
type testOptionFunc func(*testReconciler)

func (f testOptionFunc) apply(tr *testReconciler) {
	f(tr)
}

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
//   - GatewayConfigBuilder: Returns empty config, no errors
//   - GatewayApplierDeleter: All operations succeed
//   - GatewayProber: Gateway is ready
//   - FlowHealthProber: Flow is healthy
//   - IstioStatusChecker: Istio is not active
//   - OverridesHandler: Returns empty overrides
//   - PipelineLock: Lock operations succeed
//   - PipelineSyncer: Sync operations succeed
//   - PipelineValidator: All validations pass
//   - ErrorToMessageConverter: Standard converter
func newTestReconciler(client client.Client, opts ...interface{}) (*testReconciler, func(*testing.T)) {
	registry := newMockRegistry()

	tr := &testReconciler{
		mockRegistry: registry,
		assertMocks:  registry.assertAll,
	}

	// Set up default mocks
	gatewayConfigBuilder := &mocks.GatewayConfigBuilder{}
	gatewayConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil).Maybe()

	gatewayApplierDeleter := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	flowHealthProber := &mocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil).Maybe()

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil).Maybe()
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil).Maybe()

	pipelineSyncer := &mocks.PipelineSyncer{}
	pipelineSyncer.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Build default options with mocked dependencies
	reconcilerOpts := []Option{
		WithGlobal(config.NewGlobal(config.WithTargetNamespace("default"))),
		WithGatewayConfigBuilder(gatewayConfigBuilder),
		WithGatewayApplierDeleter(gatewayApplierDeleter),
		WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(nil)),
		WithFlowHealthProber(flowHealthProber),
		WithIstioStatusChecker(&stubs.IstioStatusChecker{IsActive: false}),
		WithOverridesHandler(overridesHandler),
		WithPipelineLock(pipelineLock),
		WithPipelineSyncer(pipelineSyncer),
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
	tr.Reconciler = New(client, reconcilerOpts...)

	// Now apply test options that need access to the initialized Reconciler
	for _, testOpt := range testOpts {
		testOpt.apply(tr)
	}

	return tr, tr.assertMocks
}

// WithGatewayConfigBuilderAssert registers a GatewayConfigBuilder mock for auto-assertion.
//
// Behavior:
//   - If mock has .Times(), .Once(), etc. → calls AssertExpectations
//   - If mock has no expectations → calls AssertNotCalled for "Build"
//
// Example with expectations:
//
//	gatewayConfigMock := &mocks.GatewayConfigBuilder{}
//	gatewayConfigMock.On("Build", mock.Anything, containsPipeline(pipeline), mock.Anything).
//	    Return(&common.Config{}, nil, nil).Once()
//	sut, assertMocks := newTestReconciler(fakeClient, WithGatewayConfigBuilderAssert(gatewayConfigMock))
//	result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
//	assertMocks(t) // Calls gatewayConfigMock.AssertExpectations(t)
//
// Example without expectations (should not be called):
//
//	gatewayConfigMock := &mocks.GatewayConfigBuilder{}
//	// No On() setup means it should not be called
//	sut, assertMocks := newTestReconciler(fakeClient, WithGatewayConfigBuilderAssert(gatewayConfigMock))
//	result := reconcileAndGet(t, fakeClient, sut, pipeline.Name)
//	assertMocks(t) // Calls gatewayConfigMock.AssertNotCalled(t, "Build")
func WithGatewayConfigBuilderAssert(mockBuilder *mocks.GatewayConfigBuilder) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.gatewayConfigBuilder = mockBuilder
		registerMockForAssertion(tr.mockRegistry, mockBuilder, []string{"Build"})
	})
}

// WithGatewayApplierDeleterAssert registers a GatewayApplierDeleter mock for auto-assertion.
// Checks both ApplyResources and DeleteResources methods.
func WithGatewayApplierDeleterAssert(mockApplierDeleter *mocks.GatewayApplierDeleter) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.gatewayApplierDeleter = mockApplierDeleter
		registerMockForAssertion(tr.mockRegistry, mockApplierDeleter, []string{"ApplyResources", "DeleteResources"})
	})
}

// WithFlowHealthProberAssert registers a FlowHealthProber mock for auto-assertion.
func WithFlowHealthProberAssert(mockProber *mocks.FlowHealthProber) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.flowHealthProber = mockProber
		registerMockForAssertion(tr.mockRegistry, mockProber, []string{"Probe"})
	})
}

// WithGatewayProberAssert registers a gateway Prober mock for auto-assertion.
// Note: This works with commonstatus.Prober interface implementations.
func WithGatewayProberAssert(mockProber interface {
	commonstatus.Prober
	AssertExpectations(t mock.TestingT) bool
}) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.gatewayProber = mockProber
		registerMockForAssertion(tr.mockRegistry, mockProber, []string{"Probe"})
	})
}

// WithOverridesHandlerAssert registers an OverridesHandler mock for auto-assertion.
func WithOverridesHandlerAssert(mockHandler *mocks.OverridesHandler) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.overridesHandler = mockHandler
		registerMockForAssertion(tr.mockRegistry, mockHandler, []string{"LoadOverrides"})
	})
}

// WithPipelineLockAssert registers a PipelineLock mock for auto-assertion.
func WithPipelineLockAssert(mockLock *mocks.PipelineLock) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.pipelineLock = mockLock
		registerMockForAssertion(tr.mockRegistry, mockLock, []string{"TryAcquireLock", "IsLockHolder"})
	})
}

// WithPipelineSyncerAssert registers a PipelineSyncer mock for auto-assertion.
func WithPipelineSyncerAssert(mockSyncer *mocks.PipelineSyncer) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.Reconciler.pipelineSync = mockSyncer
		registerMockForAssertion(tr.mockRegistry, mockSyncer, []string{"Sync"})
	})
}

// registerMockForAssertion is a helper that checks if a mock has expectations and registers it appropriately.
func registerMockForAssertion(registry *mockRegistry, mockObj interface{ AssertExpectations(t mock.TestingT) bool }, methods []string) {
	// Try to get the underlying mock.Mock to inspect expectations
	type mockWithCalls interface {
		ExpectedCalls() []*mock.Call
	}

	if mockWithCallsObj, ok := mockObj.(mockWithCalls); ok {
		expectedCalls := mockWithCallsObj.ExpectedCalls()
		if len(expectedCalls) > 0 {
			hasExpectations := false
			for _, call := range expectedCalls {
				// If Repeatability is > 0 or -1 (unlimited), it has expectations
				if call.Repeatability > 0 || call.Repeatability == -1 {
					hasExpectations = true
					break
				}
			}

			if hasExpectations {
				registry.registerWithExpectations(mockObj)
				return
			}
		}
	}

	// No expectations - should not be called
	registry.registerWithoutExpectations(mockObj, methods)
}
