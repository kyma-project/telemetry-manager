package kubernetes

import (
	"context"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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

func CreateIfNotExistsConfigMap(ctx context.Context, c client.Client, desired *corev1.ConfigMap) error {
	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &corev1.ConfigMap{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}
	return nil
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
	mergeKubectlAnnotations(&desired.Spec.Template.ObjectMeta, existing.Spec.Template.ObjectMeta)
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
	mergeKubectlAnnotations(&desired.Spec.Template.ObjectMeta, existing.Spec.Template.ObjectMeta)
	mergeChecksumAnnotations(&desired.Spec.Template.ObjectMeta, existing.Spec.Template.ObjectMeta)
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

func CreateOrUpdateLokiLogPipeline(ctx context.Context, c client.Client, desired *telemetryv1alpha1.LogPipeline) error {
	var existing telemetryv1alpha1.LogPipeline
	err := c.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return c.Create(ctx, desired)
	}

	mergeMetadata(&desired.ObjectMeta, existing.ObjectMeta)
	mergeFinalizers(desired, existing.ObjectMeta.Finalizers)

	return c.Update(ctx, desired)
}

func DeleteLokiLogPipeline(ctx context.Context, c client.Client, lokiLogPipeline *telemetryv1alpha1.LogPipeline) error {
	err := c.Delete(ctx, lokiLogPipeline)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func mergeMetadata(new *metav1.ObjectMeta, old metav1.ObjectMeta) {
	new.ResourceVersion = old.ResourceVersion

	new.SetLabels(mergeMaps(new.Labels, old.Labels))
	new.SetAnnotations(mergeMaps(new.Annotations, old.Annotations))
	new.SetOwnerReferences(mergeOwnerReferences(new.OwnerReferences, old.OwnerReferences))
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

func mergeMaps(new map[string]string, old map[string]string) map[string]string {
	return mergeMapsByPrefix(new, old, "")
}

func mergeKubectlAnnotations(new *metav1.ObjectMeta, old metav1.ObjectMeta) {
	new.SetAnnotations(mergeMapsByPrefix(new.Annotations, old.Annotations, "kubectl.kubernetes.io/"))
}

func mergeChecksumAnnotations(new *metav1.ObjectMeta, old metav1.ObjectMeta) {
	new.SetAnnotations(mergeMapsByPrefix(new.Annotations, old.Annotations, "checksum/"))
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

func mergeFinalizers(newObject client.Object, oldFinalizers []string) {
	for _, v := range oldFinalizers {
		controllerutil.AddFinalizer(newObject, v)
	}
}

func GetOrCreateConfigMap(ctx context.Context, c client.Client, name types.NamespacedName) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace}}
	err := c.Get(ctx, client.ObjectKeyFromObject(&cm), &cm)
	if err == nil {
		return cm, nil
	}
	if apierrors.IsNotFound(err) {
		err = c.Create(ctx, &cm)
		if err == nil {
			return cm, nil
		}
	}
	return corev1.ConfigMap{}, err
}

func GetOrCreateSecret(ctx context.Context, c client.Client, name types.NamespacedName) (corev1.Secret, error) {
	secret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace}}
	err := c.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)
	if err == nil {
		return secret, nil
	}
	if apierrors.IsNotFound(err) {
		err = c.Create(ctx, &secret)
		if err == nil {
			return secret, nil
		}
	}
	return corev1.Secret{}, err
}
