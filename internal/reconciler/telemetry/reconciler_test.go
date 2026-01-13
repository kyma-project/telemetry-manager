package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1beta1.AddToScheme(scheme)
	_ = operatorv1beta1.AddToScheme(scheme)

	telemetry := operatorv1beta1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: operatorv1beta1.TelemetrySpec{},
	}
	fakeClient := fake.NewClientBuilder().
		WithObjects(&telemetry).
		WithStatusSubresource(&telemetry).
		WithScheme(scheme).
		Build()

	overridesHandlerStub := &mocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", t.Context()).Return(&overrides.Config{}, nil)

	sut := Reconciler{
		Client:           fakeClient,
		config:           Config{},
		healthCheckers:   healthCheckers{},
		overridesHandler: overridesHandlerStub,
	}

	_, err := sut.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test",
		},
	})
	require.NoError(t, err)
}
