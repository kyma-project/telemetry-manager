package kubernetes

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TryAcquireLock(ctx context.Context, client client.Client, lockName types.NamespacedName, owner metav1.Object) error {
	var lock corev1.ConfigMap
	if err := client.Get(ctx, lockName, &lock); err != nil {
		if apierrors.IsNotFound(err) {
			return createLock(ctx, client, lockName, owner)
		}
		return fmt.Errorf("failed to get lock: %v", err)
	}

	for _, ref := range lock.GetOwnerReferences() {
		if ref.Name == owner.GetName() && ref.UID == owner.GetUID() {
			return nil
		}
	}

	return errors.New("lock is already acquired by another TracePipeline")
}

func createLock(ctx context.Context, client client.Client, name types.NamespacedName, owner metav1.Object) error {
	lock := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	if err := controllerutil.SetControllerReference(owner, &lock, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}
	if err := client.Create(ctx, &lock); err != nil {
		return fmt.Errorf("failed to create lock: %v", err)
	}
	return nil
}
