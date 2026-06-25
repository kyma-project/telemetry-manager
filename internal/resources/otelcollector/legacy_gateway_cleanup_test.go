package otelcollector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

func TestDeleteLegacyGatewayResources(t *testing.T) {
	tests := []struct {
		name            string
		istioActive     bool
		gatewayName     string
		createResources bool
	}{
		{
			name:            "delete with istio",
			istioActive:     true,
			gatewayName:     "telemetry-trace-gateway",
			createResources: true,
		},
		{
			name:            "delete without istio",
			istioActive:     false,
			gatewayName:     "telemetry-metric-gateway",
			createResources: true,
		},
		{
			name:            "delete when resources don't exist",
			istioActive:     false,
			gatewayName:     "non-existent-gateway",
			createResources: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			// Define all resources that should be created and verified
			objectMeta := metav1.ObjectMeta{Name: tt.gatewayName, Namespace: "kyma-system"}
			metricsServiceMeta := metav1.ObjectMeta{Name: names.MetricsServiceName(tt.gatewayName), Namespace: "kyma-system"}

			resources := []client.Object{
				// Core resources
				&appsv1.Deployment{ObjectMeta: objectMeta},
				&corev1.Secret{ObjectMeta: objectMeta},
				&corev1.ConfigMap{ObjectMeta: objectMeta},
				// RBAC resources
				&rbacv1.ClusterRoleBinding{ObjectMeta: objectMeta},
				&rbacv1.ClusterRole{ObjectMeta: objectMeta},
				&rbacv1.RoleBinding{ObjectMeta: objectMeta},
				&rbacv1.Role{ObjectMeta: objectMeta},
				&corev1.ServiceAccount{ObjectMeta: objectMeta},
				// Metrics service
				&corev1.Service{ObjectMeta: metricsServiceMeta},
			}

			// Add NetworkPolicy with labels
			networkPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.gatewayName + "-network-policy",
					Namespace: "kyma-system",
					Labels: map[string]string{
						commonresources.LabelKeyK8sName: tt.gatewayName,
					},
				},
			}
			resources = append(resources, networkPolicy)

			// Add PeerAuthentication if Istio is active
			if tt.istioActive {
				peerAuth := &istiosecurityclientv1.PeerAuthentication{ObjectMeta: objectMeta}
				resources = append(resources, peerAuth)
			}

			if tt.createResources {
				for _, resource := range resources {
					require.NoError(t, fakeClient.Create(context.Background(), resource))
				}
			}

			err := DeleteLegacyGatewayResources(context.Background(), fakeClient, "kyma-system", tt.gatewayName, tt.istioActive)
			require.NoError(t, err)

			// Verify all resources are deleted if they were created
			if tt.createResources {
				for _, resource := range resources {
					err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(resource), resource)
					require.True(t, apierrors.IsNotFound(err), "resource %T %s should be deleted", resource, client.ObjectKeyFromObject(resource))
				}
			}
		})
	}
}
