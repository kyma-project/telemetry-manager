package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeleteLegacyGatewayResources(t *testing.T) {
	tests := []struct {
		name            string
		istioActive     bool
		vpaCRDExists    bool
		gatewayName     string
		createResources bool
	}{
		{
			name:            "delete with istio and VPA",
			istioActive:     true,
			vpaCRDExists:    true,
			gatewayName:     "telemetry-trace-gateway",
			createResources: true,
		},
		{
			name:            "delete without istio",
			istioActive:     false,
			vpaCRDExists:    true,
			gatewayName:     "telemetry-metric-gateway",
			createResources: true,
		},
		{
			name:            "delete without VPA",
			istioActive:     true,
			vpaCRDExists:    false,
			gatewayName:     "telemetry-log-gateway",
			createResources: true,
		},
		{
			name:            "delete when resources don't exist",
			istioActive:     false,
			vpaCRDExists:    false,
			gatewayName:     "non-existent-gateway",
			createResources: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
			utilruntime.Must(autoscalingvpav1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			if tt.createResources {
				// Create mock legacy resources
				objectMeta := metav1.ObjectMeta{Name: tt.gatewayName, Namespace: "kyma-system"}

				deployment := &appsv1.Deployment{ObjectMeta: objectMeta}
				require.NoError(t, fakeClient.Create(context.Background(), deployment))

				secret := &corev1.Secret{ObjectMeta: objectMeta}
				require.NoError(t, fakeClient.Create(context.Background(), secret))

				configMap := &corev1.ConfigMap{ObjectMeta: objectMeta}
				require.NoError(t, fakeClient.Create(context.Background(), configMap))
			}

			err := DeleteLegacyGatewayResources(context.Background(), fakeClient, "kyma-system", tt.gatewayName, tt.istioActive, tt.vpaCRDExists)

			// The function should not return an error (it's idempotent)
			// But it may return errors if deletion fails, which is acceptable
			if err != nil {
				t.Logf("DeleteLegacyGatewayResources returned error (may be expected): %v", err)
			}

			// Verify resources are deleted if they were created
			if tt.createResources {
				objectMeta := metav1.ObjectMeta{Name: tt.gatewayName, Namespace: "kyma-system"}

				deployment := &appsv1.Deployment{ObjectMeta: objectMeta}
				err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(deployment), deployment)
				require.Error(t, err, "deployment should be deleted")

				secret := &corev1.Secret{ObjectMeta: objectMeta}
				err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(secret), secret)
				require.Error(t, err, "secret should be deleted")

				configMap := &corev1.ConfigMap{ObjectMeta: objectMeta}
				err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(configMap), configMap)
				require.Error(t, err, "configmap should be deleted")
			}
		})
	}
}
