package k8s

import (
	"context"
	"strings"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdateClusterRoleBinding(ctx context.Context, c client.Client, desired *rbacv1.ClusterRoleBinding) error {
	desired.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))
	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateClusterRole(ctx context.Context, c client.Client, desired *rbacv1.ClusterRole) error {
	desired.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateRoleBinding(ctx context.Context, c client.Client, desired *rbacv1.RoleBinding) error {
	desired.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateRole(ctx context.Context, c client.Client, desired *rbacv1.Role) error {
	desired.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("Role"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateServiceAccount(ctx context.Context, c client.Client, desired *corev1.ServiceAccount) error {
	desired.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateConfigMap(ctx context.Context, c client.Client, desired *corev1.ConfigMap) error {
	desired.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateNetworkPolicy(ctx context.Context, c client.Client, desired *networkingv1.NetworkPolicy) error {
	desired.SetGroupVersionKind(networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateSecret(ctx context.Context, c client.Client, desired *corev1.Secret) error {
	desired.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateDeployment(ctx context.Context, c client.Client, desired *appsv1.Deployment) error {
	desired.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateDaemonSet(ctx context.Context, c client.Client, desired *appsv1.DaemonSet) error {
	desired.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateService(ctx context.Context, c client.Client, desired *corev1.Service) error {
	desired.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdatePeerAuthentication(ctx context.Context, c client.Client, desired *istiosecurityclientv1.PeerAuthentication) error {
	desired.SetGroupVersionKind(istiosecurityclientv1.SchemeGroupVersion.WithKind("PeerAuthentication"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func CreateOrUpdateValidatingWebhookConfiguration(ctx context.Context, c client.Client, desired *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	desired.SetGroupVersionKind(admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"))

	return c.Patch(ctx, desired, client.Apply, &client.PatchOptions{
		FieldManager: "telemetry-manager",
		Force:        pointer.Bool(true),
	})
}

func mergeMetadata(newMeta *metav1.ObjectMeta, oldMeta metav1.ObjectMeta) {
	newMeta.ResourceVersion = oldMeta.ResourceVersion

	newMeta.SetLabels(mergeMaps(newMeta.Labels, oldMeta.Labels))
	newMeta.SetAnnotations(mergeMaps(newMeta.Annotations, oldMeta.Annotations))
	newMeta.SetOwnerReferences(mergeOwnerReferences(newMeta.OwnerReferences, oldMeta.OwnerReferences))
}

func ownerSliceContains(owners []metav1.OwnerReference, owner metav1.OwnerReference) bool {
	for _, o := range owners {
		if o.UID == owner.UID {
			return true
		}
	}

	return false
}

// merges two owner references slices
func mergeOwnerReferences(newOwners []metav1.OwnerReference, oldOwners []metav1.OwnerReference) []metav1.OwnerReference {
	merged := oldOwners

	for _, o := range newOwners {
		if ownerSliceContains(oldOwners, o) {
			continue
		}

		merged = append(merged, o)
	}

	return merged
}

func mergeMaps(newMap map[string]string, oldMap map[string]string) map[string]string {
	return mergeMapsByPrefix(newMap, oldMap, "")
}

func mergePodAnnotations(newMeta *metav1.ObjectMeta, oldMeta metav1.ObjectMeta) {
	newMeta.SetAnnotations(mergeMapsByPrefix(newMeta.Annotations, oldMeta.Annotations, "kubectl.kubernetes.io/"))
	newMeta.SetAnnotations(mergeMapsByPrefix(newMeta.Annotations, oldMeta.Annotations, commonresources.AnnotationKeyChecksumConfig))
	newMeta.SetAnnotations(mergeMapsByPrefix(newMeta.Annotations, oldMeta.Annotations, "istio-operator.kyma-project.io/restartedAt"))
}

func mergeMapsByPrefix(newMap map[string]string, oldMap map[string]string, prefix string) map[string]string {
	if oldMap == nil {
		oldMap = make(map[string]string)
	}

	if newMap == nil {
		newMap = make(map[string]string)
	}

	for k, v := range oldMap {
		_, hasValue := newMap[k]
		if strings.HasPrefix(k, prefix) && !hasValue {
			newMap[k] = v
		}
	}

	return newMap
}

func DeleteObject(ctx context.Context, c client.Client, obj client.Object) error {
	err := c.Delete(ctx, obj)
	return client.IgnoreNotFound(err)
}
