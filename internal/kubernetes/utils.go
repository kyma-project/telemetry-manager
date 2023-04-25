package kubernetes

import (
	"context"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func mergeMetadata(new *metav1.ObjectMeta, old metav1.ObjectMeta) {
	new.ResourceVersion = old.ResourceVersion

	new.SetLabels(mergeMaps(new.Labels, old.Labels))
	new.SetAnnotations(mergeMaps(new.Annotations, old.Annotations))
	new.SetOwnerReferences(mergeOwnerReferences(new.OwnerReferences, old.OwnerReferences))
}

func EnqueueRequestForOwnerFuncs(log logr.Logger) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			enqueueRequestsForOwners(log, evt.Object, q)
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			enqueueRequestsForOwners(log, evt.Object, q)
		},
		GenericFunc: func(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
			enqueueRequestsForOwners(log, evt.Object, q)
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			oldOwners := getOwnersFromObject(e.ObjectOld)
			newOwners := getOwnersFromObject(e.ObjectNew)

			for _, newOwner := range newOwners {
				if ownerSliceContains(oldOwners, newOwner) && hasController(newOwners) {
					continue
				}

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: e.ObjectNew.GetNamespace(),
						Name:      newOwner.Name,
					},
				}
				q.Add(req)
				log.V(1).Info("Enqueued reconcile request for owner", "owner", req.NamespacedName)
			}
		},
	}
}

func hasController(owners []metav1.OwnerReference) bool {
	for _, owner := range owners {
		equal := reflect.DeepEqual(owner.Controller, (func() *bool { b := true; return &b }()))
		if equal {
			return true
		}
	}
	return false
}

func enqueueRequestsForOwners(log logr.Logger, obj metav1.Object, q workqueue.RateLimitingInterface) {
	owners := getOwnersFromObject(obj)

	for _, owner := range owners {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      owner.Name,
			},
		}
		q.Add(req)
		log.V(1).Info("Enqueued reconcile request for owner", "owner", req.NamespacedName)
	}
}

func getOwnersFromObject(obj metav1.Object) []metav1.OwnerReference {
	if obj == nil {
		return nil
	}

	return obj.GetOwnerReferences()
}

func ownerSliceContains(owners []metav1.OwnerReference, owner metav1.OwnerReference) bool {
	for _, o := range owners {
		if o.UID == owner.UID {
			return true
		}
	}
	return false
}

// merges two owner references slices. Since only one owner can have the controller flag, we assume that the newOwners slice contains the actual controller and clear the flag on all owners in oldOwners.
func mergeOwnerReferences(newOwners []metav1.OwnerReference, oldOwners []metav1.OwnerReference) []metav1.OwnerReference {
	merged := oldOwners
	//notOwner := false

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
