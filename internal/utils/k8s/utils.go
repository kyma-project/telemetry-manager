package k8s

import (
	"context"
	"strings"

	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

func CreateOrUpdateClusterRoleBinding(ctx context.Context, c client.Client, desired *rbacv1.ClusterRoleBinding) error {
	var existing rbacv1.ClusterRoleBinding

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateClusterRole(ctx context.Context, c client.Client, desired *rbacv1.ClusterRole) error {
	var existing rbacv1.ClusterRole

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateRoleBinding(ctx context.Context, c client.Client, desired *rbacv1.RoleBinding) error {
	var existing rbacv1.RoleBinding

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateRole(ctx context.Context, c client.Client, desired *rbacv1.Role) error {
	var existing rbacv1.Role

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateServiceAccount(ctx context.Context, c client.Client, desired *corev1.ServiceAccount) error {
	var existing corev1.ServiceAccount

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateConfigMap(ctx context.Context, c client.Client, desired *corev1.ConfigMap) error {
	var existing corev1.ConfigMap

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateNetworkPolicy(ctx context.Context, c client.Client, desired *networkingv1.NetworkPolicy) error {
	var existing networkingv1.NetworkPolicy

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateSecret(ctx context.Context, c client.Client, desired *corev1.Secret) error {
	var existing corev1.Secret

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mutated := existing.DeepCopy()
	mergeMetadata(&desired.ObjectMeta, mutated.ObjectMeta)

	if apiequality.Semantic.DeepEqual(mutated, desired) {
		return nil
	}

	return c.Update(ctx, desired)
}

func CreateOrUpdateDeployment(ctx context.Context, c client.Client, desired *appsv1.Deployment) error {
	var existing appsv1.Deployment

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)
	mergePodAnnotations(&desired.Spec.Template.ObjectMeta, existing.Spec.Template.ObjectMeta)

	return c.Update(ctx, desired)
}

func CreateOrUpdateDaemonSet(ctx context.Context, c client.Client, desired *appsv1.DaemonSet) error {
	var existing appsv1.DaemonSet

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)
	mergePodAnnotations(&desired.Spec.Template.ObjectMeta, existing.Spec.Template.ObjectMeta)

	return c.Update(ctx, desired)
}

func CreateOrUpdateService(ctx context.Context, c client.Client, desired *corev1.Service) error {
	var existing corev1.Service

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	// Apply immutable fields from the existing service.
	desired.Spec.IPFamilies = existing.Spec.IPFamilies
	desired.Spec.IPFamilyPolicy = existing.Spec.IPFamilyPolicy
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.Spec.ClusterIPs = existing.Spec.ClusterIPs

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)

	return c.Update(ctx, desired)
}

func CreateOrUpdatePeerAuthentication(ctx context.Context, c client.Client, desired *istiosecurityclientv1.PeerAuthentication) error {
	var existing istiosecurityclientv1.PeerAuthentication

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)

	return c.Update(ctx, desired)
}

func CreateOrUpdateDestinationRule(ctx context.Context, c client.Client, desired *istionetworkingclientv1.DestinationRule) error {
	var existing istionetworkingclientv1.DestinationRule

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)

	return c.Update(ctx, desired)
}
func CreateOrUpdateValidatingWebhookConfiguration(ctx context.Context, c client.Client, desired *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	var existing admissionregistrationv1.ValidatingWebhookConfiguration

	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)

	return c.Update(ctx, desired)
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

func DeleteObjectsByLabelSelector(ctx context.Context, c client.Client, objList client.ObjectList, labelSelector map[string]string) error {
	listOptions := []client.ListOption{
		client.MatchingLabels(labelSelector),
	}

	err := c.List(ctx, objList, listOptions...)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	items, err := meta.ExtractList(objList)
	if err != nil {
		return err
	}

	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			continue
		}

		err = c.Delete(ctx, obj)
		if err != nil {
			return client.IgnoreNotFound(err)
		}
	}

	return nil
}
