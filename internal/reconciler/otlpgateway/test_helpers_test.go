package otlpgateway

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

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

func (r *mockRegistry) registerWithExpectations(m interface{ AssertExpectations(t mock.TestingT) bool }) {
	r.mocksWithExpectations = append(r.mocksWithExpectations, m)
}

func (r *mockRegistry) assertAll(t *testing.T) {
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
	apply(tr *testReconciler)
}

// testOptionFunc wraps a function to implement testOption
type testOptionFunc func(*testReconciler)

func (f testOptionFunc) apply(tr *testReconciler) {
	f(tr)
}

func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))
	require.NoError(t, istiosecurityclientv1.AddToScheme(scheme))

	kymaSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyma-system",
		},
	}

	kubeSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
			UID:  "test-cluster-id",
		},
	}

	allObjs := append([]client.Object{kymaSystemNamespace, kubeSystemNamespace}, objs...)

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(allObjs...).WithStatusSubresource(objs...).Build()
}

// newTestReconciler creates a Reconciler with default empty mocks. Pass With* options to override
// specific dependencies, or withXxxAssert options to register mocks for automatic assertion.
func newTestReconciler(fakeClient client.Client, opts ...any) (*testReconciler, func(*testing.T)) {
	registry := newMockRegistry()

	tr := &testReconciler{
		mockRegistry: registry,
		assertMocks:  registry.assertAll,
	}

	cb := &mocks.OTLPGatewayConfigBuilder{}
	cb.On("Build", mock.Anything, mock.Anything).Return(&common.Config{}, common.EnvVars{}, nil).Maybe()

	gad := &mocks.GatewayApplierDeleter{}
	gad.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	gad.On("DeleteResources", mock.Anything, mock.Anything, false, false).Return(nil).Maybe()

	r := &Reconciler{
		Client:                fakeClient,
		globals:               config.NewGlobal(config.WithTargetNamespace("kyma-system")),
		gatewayApplierDeleter: gad,
		configBuilder:         cb,
		istioStatusChecker:    &stubs.IstioStatusChecker{IsActive: false},
		vpaStatusChecker:      &stubs.VpaStatusChecker{CRDExists: false},
		nodeSizeTracker:       &stubs.NodeSizeTracker{MaxMemory: resource.Quantity{}},
	}

	// Apply production options first to build the reconciler
	for _, opt := range opts {
		if v, ok := opt.(Option); ok {
			v(r)
		}
	}

	tr.Reconciler = r

	// Apply test options after Reconciler is set so they can access and override its fields
	for _, opt := range opts {
		if v, ok := opt.(testOption); ok {
			v.apply(tr)
		}
	}

	return tr, tr.assertMocks
}

func newReconcileRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: "kyma-system",
		},
	}
}

// withGatewayApplierDeleterAssert injects a GatewayApplierDeleter mock and registers it for auto-assertion.
func withGatewayApplierDeleterAssert(m *mocks.GatewayApplierDeleter) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.gatewayApplierDeleter = m
		tr.mockRegistry.registerWithExpectations(m)
	})
}

// withConfigBuilderAssert injects an OTLPGatewayConfigBuilder mock and registers it for auto-assertion.
func withConfigBuilderAssert(m *mocks.OTLPGatewayConfigBuilder) testOption {
	return testOptionFunc(func(tr *testReconciler) {
		tr.configBuilder = m
		tr.mockRegistry.registerWithExpectations(m)
	})
}
