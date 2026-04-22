package telemetry

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
)

// newTestScheme creates a runtime.Scheme with all types needed by the telemetry reconciler.
func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))
	require.NoError(t, operatorv1beta1.AddToScheme(scheme))

	return scheme
}

// newTestClient creates a fake Kubernetes client for testing with the given objects.
func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	scheme := newTestScheme(t)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()
}

// reconcileAndGet performs a reconciliation.
func reconcileAndGet(t *testing.T, sut *Reconciler, name, namespace string) {
	t.Helper()

	sut.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	})
}

// newTestReconciler creates a Reconciler with all dependencies mocked by default.
//
// Default behavior:
//   - Config: target namespace "kyma-system"
//   - OverridesHandler: returns empty config, no errors
//   - HealthCheckers: all return healthy condition
//   - SelfMonitorApplierDeleter: nil (not used unless explicitly set)
//   - Scheme: includes clientgo, telemetryv1beta1, operatorv1beta1
func newTestReconciler(t *testing.T, fakeClient client.Client) *Reconciler {
	t.Helper()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil).Maybe()

	healthChecker := &mocks.ComponentHealthChecker{}
	healthChecker.On("Check", mock.Anything, mock.Anything).Return(&metav1.Condition{
		Type:   "Healthy",
		Status: metav1.ConditionTrue,
		Reason: "ComponentHealthy",
	}, nil).Maybe()

	return &Reconciler{
		Client: fakeClient,
		config: Config{
			Global: config.NewGlobal(config.WithTargetNamespace("kyma-system")),
		},
		scheme: newTestScheme(t),
		healthCheckers: healthCheckers{
			logs:    healthChecker,
			metrics: healthChecker,
			traces:  healthChecker,
		},
		overridesHandler: overridesHandler,
	}
}
