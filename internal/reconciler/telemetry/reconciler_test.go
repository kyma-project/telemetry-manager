package telemetry

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const (
	telemetryName      = "default"
	telemetryNamespace = "kyma-system"
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

func TestReconcile_ServiceAttributesEnrichmentStrategyMetric(t *testing.T) {
	tests := []struct {
		name             string
		annotations      map[string]string
		expectedStrategy string
	}{
		{
			name:             "defaults to kyma-legacy when no annotation is set",
			annotations:      nil,
			expectedStrategy: commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy,
		},
		{
			name: "sets otel strategy when annotation is otel",
			annotations: map[string]string{
				commonresources.AnnotationKeyTelemetryServiceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
			},
			expectedStrategy: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
		},
		{
			name: "sets kyma-legacy strategy when annotation is kyma-legacy",
			annotations: map[string]string{
				commonresources.AnnotationKeyTelemetryServiceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy,
			},
			expectedStrategy: commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy,
		},
		{
			name: "defaults to kyma-legacy when annotation has invalid value",
			annotations: map[string]string{
				commonresources.AnnotationKeyTelemetryServiceEnrichment: "invalid",
			},
			expectedStrategy: commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.ServiceAttributesEnrichmentStrategy.Reset()

			telemetryCR := &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:        telemetryName,
					Namespace:   telemetryNamespace,
					Annotations: tt.annotations,
				},
			}
			fakeClient := newTestClient(t, telemetryCR)
			sut := newTestReconciler(t, fakeClient)

			reconcileAndGet(t, sut, telemetryName, telemetryNamespace)

			activeValue := testutil.ToFloat64(metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues(tt.expectedStrategy))
			require.Equal(t, 1.0, activeValue, "expected strategy %q to be active (1)", tt.expectedStrategy)

			// Verify the other strategy is set to 0
			otherStrategy := commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			if tt.expectedStrategy == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				otherStrategy = commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy
			}

			inactiveValue := testutil.ToFloat64(metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues(otherStrategy))
			require.Equal(t, 0.0, inactiveValue, "expected strategy %q to be inactive (0)", otherStrategy)
		})
	}
}

func TestReconcile_ServiceAttributesEnrichmentStrategyMetric_Switch(t *testing.T) {
	metrics.ServiceAttributesEnrichmentStrategy.Reset()

	// First reconcile with kyma-legacy (default, no annotation)
	telemetryCR := &operatorv1beta1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      telemetryName,
			Namespace: telemetryNamespace,
		},
	}
	fakeClient := newTestClient(t, telemetryCR)
	sut := newTestReconciler(t, fakeClient)

	reconcileAndGet(t, sut, telemetryName, telemetryNamespace)

	require.Equal(t, 1.0, testutil.ToFloat64(metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues("kyma-legacy")))
	require.Equal(t, 0.0, testutil.ToFloat64(metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues("otel")))

	// Now switch to otel by updating the Telemetry CR annotation
	telemetryCR.Annotations = map[string]string{
		commonresources.AnnotationKeyTelemetryServiceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
	}
	fakeClient = newTestClient(t, telemetryCR)
	sut.Client = fakeClient

	reconcileAndGet(t, sut, telemetryName, telemetryNamespace)

	require.Equal(t, 0.0, testutil.ToFloat64(metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues("kyma-legacy")))
	require.Equal(t, 1.0, testutil.ToFloat64(metrics.ServiceAttributesEnrichmentStrategy.WithLabelValues("otel")))
}
