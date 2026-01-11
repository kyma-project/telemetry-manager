package logpipeline

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
)

// reconcileResult holds the result of a reconciliation operation for test assertions.
type reconcileResult struct {
	result ctrl.Result
	err    error
}

// newTestClient creates a fake Kubernetes client for testing with the given objects.
// The client is configured with proper schemes and status subresources.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(objs...).Build()
}

// reconcile performs a reconciliation and returns the result and any error.
// It's a helper to reduce boilerplate in tests.
func reconcile(t *testing.T, reconciler *Reconciler, pipelineName string) reconcileResult {
	res, recErr := reconciler.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: pipelineName},
	})

	return reconcileResult{result: res, err: recErr}
}

// newTestReconciler creates a Reconciler with all dependencies mocked by default.
// Use the production Option functions to override specific dependencies.
//
// Default behavior:
//   - OverridesHandler: Returns empty config, no pausing
//   - PipelineSyncer: Lock acquisition succeeds
//   - Reconcilers: None registered by default (provide via options)
func newTestReconciler(client client.Client, opts ...Option) *Reconciler {
	// Set up default mocks
	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil)

	pipelineSync := &logpipelinemocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	// Build default options with mocked dependencies
	allOpts := []Option{
		WithOverridesHandler(overridesHandler),
		WithPipelineSyncer(pipelineSync),
	}

	// Merge default options with provided options (provided options will override defaults)
	allOpts = append(allOpts, opts...)

	return New(client, allOpts...)
}
