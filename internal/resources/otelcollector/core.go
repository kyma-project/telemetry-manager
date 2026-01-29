package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

// applyCommonResources applies resources to gateway and agent deployment node
func applyCommonResources(ctx context.Context, c client.Client, name types.NamespacedName, componentType string, rbac rbac, ingressAllowedPorts []int32) error {
	// Create service account before RBAC resources
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, makeServiceAccount(name, componentType)); err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	// Create RBAC resources in the following order: cluster role, cluster role binding, role, role binding
	if rbac.clusterRole != nil {
		// Deep copy the rbac.clusterRole object to avoid modifying the original object (e.g. populating the resourceVersion with a value)
		// So that the original rbac.clusterRole can be re-used again for re-creating the cluster role when needed
		if err := k8sutils.CreateOrUpdateClusterRole(ctx, c, rbac.clusterRole.DeepCopy()); err != nil {
			return fmt.Errorf("failed to create cluster role: %w", err)
		}
	}

	if rbac.clusterRoleBinding != nil {
		// Deep copy the rbac.clusterRoleBinding object to avoid modifying the original object (e.g. populating the resourceVersion with a value)
		// So that the original rbac.clusterRoleBinding can be re-used again for re-creating the cluster role binding when needed
		if err := k8sutils.CreateOrUpdateClusterRoleBinding(ctx, c, rbac.clusterRoleBinding.DeepCopy()); err != nil {
			return fmt.Errorf("failed to create cluster role binding: %w", err)
		}
	}

	if rbac.role != nil {
		// Deep copy the rbac.role object to avoid modifying the original object (e.g. populating the resourceVersion with a value)
		// So that the original rbac.role can be re-used again for re-creating the role when needed
		if err := k8sutils.CreateOrUpdateRole(ctx, c, rbac.role.DeepCopy()); err != nil {
			return fmt.Errorf("failed to create role: %w", err)
		}
	}

	if rbac.roleBinding != nil {
		// Deep copy the rbac.roleBinding object to avoid modifying the original object (e.g. populating the resourceVersion with a value)
		// So that the original rbac.roleBinding can be re-used again for re-creating the roleBinding when needed
		if err := k8sutils.CreateOrUpdateRoleBinding(ctx, c, rbac.roleBinding.DeepCopy()); err != nil {
			return fmt.Errorf("failed to create role binding: %w", err)
		}
	}

	if err := k8sutils.CreateOrUpdateService(ctx, c, makeMetricsService(name, componentType)); err != nil {
		return fmt.Errorf("failed to create metrics service: %w", err)
	}

	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, commonresources.MakeNetworkPolicy(name, ingressAllowedPorts, commonresources.MakeDefaultLabels(name.Name, componentType), commonresources.MakeDefaultSelectorLabels(name.Name))); err != nil {
		return fmt.Errorf("failed to create network policy: %w", err)
	}

	return nil
}

func deleteCommonResources(ctx context.Context, c client.Client, name types.NamespacedName) error {
	objectMeta := metav1.ObjectMeta{
		Name:      name.Name,
		Namespace: name.Namespace,
	}

	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	clusterRoleBinding := rbacv1.ClusterRoleBinding{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &clusterRoleBinding); err != nil && !apierrors.IsNotFound(err) {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete cluster role binding: %w", err))
	}

	clusterRole := rbacv1.ClusterRole{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &clusterRole); err != nil && !apierrors.IsNotFound(err) {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete cluster role: %w", err))
	}

	roleBinding := rbacv1.RoleBinding{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &roleBinding); err != nil && !apierrors.IsNotFound(err) {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete role binding: %w", err))
	}

	role := rbacv1.Role{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &role); err != nil && !apierrors.IsNotFound(err) {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete role: %w", err))
	}

	serviceAccount := corev1.ServiceAccount{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &serviceAccount); err != nil && !apierrors.IsNotFound(err) {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete service account: %w", err))
	}

	metricsService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.MetricsServiceName(name.Name), Namespace: name.Namespace}}
	if err := k8sutils.DeleteObject(ctx, c, &metricsService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete metrics service: %w", err))
	}

	networkPolicy := networkingv1.NetworkPolicy{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &networkPolicy); err != nil && !apierrors.IsNotFound(err) {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete network policy: %w", err))
	}

	return allErrors
}

func makeServiceAccount(name types.NamespacedName, componentType string) *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    commonresources.MakeDefaultLabels(name.Name, componentType),
		},
	}

	return &serviceAccount
}

func makeConfigMap(name types.NamespacedName, componentType string, collectorConfigYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    commonresources.MakeDefaultLabels(name.Name, componentType),
		},
		Data: map[string]string{
			configFileName: collectorConfigYAML,
		},
	}
}

func makeSecret(name types.NamespacedName, componentType string, secretData map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    commonresources.MakeDefaultLabels(name.Name, componentType),
		},
		Data: secretData,
	}
}

func makeMetricsService(name types.NamespacedName, componentType string) *corev1.Service {
	labels := commonresources.MakeDefaultLabels(name.Name, componentType)
	labels[commonresources.LabelKeyTelemetrySelfMonitor] = commonresources.LabelValueTelemetrySelfMonitor

	selectorLabels := commonresources.MakeDefaultSelectorLabels(name.Name)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.MetricsServiceName(name.Name),
			Namespace: name.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				commonresources.AnnotationKeyPrometheusScrape: "true",
				commonresources.AnnotationKeyPrometheusPort:   strconv.Itoa(int(ports.Metrics)),
				commonresources.AnnotationKeyPrometheusScheme: "http",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.Metrics,
					TargetPort: intstr.FromInt32(ports.Metrics),
				},
			},
			Selector: selectorLabels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}
