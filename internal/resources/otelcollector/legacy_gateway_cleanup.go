package otelcollector

import (
	"context"
	"errors"
	"fmt"

	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

// TODO: Remove after first roll-out
// DeleteLegacyGatewayResources removes all Kubernetes resources that were created by the old
// per-signal gateway Deployments (telemetry-trace-gateway, telemetry-metric-gateway, telemetry-log-gateway).
// This is needed for clusters upgrading from the old architecture to the centralized OTLP Gateway DaemonSet.
// The function is idempotent — it safely ignores resources that don't exist.
func DeleteLegacyGatewayResources(ctx context.Context, c client.Client, namespace string, gatewayName string, isIstioActive bool) error {
	name := types.NamespacedName{Name: gatewayName, Namespace: namespace}

	var allErrors error

	// Delete RBAC resources (ServiceAccount, ClusterRole, ClusterRoleBinding, Role, RoleBinding) and metrics Service
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{Name: gatewayName, Namespace: namespace}

	// Delete the old gateway Deployment
	deployment := appsv1.Deployment{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &deployment); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy gateway deployment %s: %w", gatewayName, err))
	}

	// Delete the gateway Secret (environment variables)
	secret := corev1.Secret{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &secret); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy gateway secret %s: %w", gatewayName, err))
	}

	// Delete the gateway ConfigMap (collector configuration)
	configMap := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &configMap); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy gateway configmap %s: %w", gatewayName, err))
	}

	// Delete NetworkPolicies by label selector
	networkPolicySelector := map[string]string{
		commonresources.LabelKeyK8sName: gatewayName,
	}

	if err := k8sutils.DeleteObjectsByLabelSelector(ctx, c, &networkingv1.NetworkPolicyList{}, networkPolicySelector); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy gateway network policies %s: %w", gatewayName, err))
	}

	// Delete PeerAuthentication if Istio is active
	if isIstioActive {
		peerAuth := istiosecurityclientv1.PeerAuthentication{ObjectMeta: objectMeta}
		if err := k8sutils.DeleteObject(ctx, c, &peerAuth); err != nil {
			allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy gateway peer authentication %s: %w", gatewayName, err))
		}
	}

	return allErrors
}
