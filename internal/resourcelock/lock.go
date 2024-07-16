package resourcelock

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

	"github.com/kyma-project/telemetry-manager/internal/errortypes"
)

var ErrMaxPipelinesExceeded = errors.New("maximum pipeline count limit exceeded")

type Checker struct {
	client    client.Client
	lockName  types.NamespacedName
	maxOwners int
}

func New(client client.Client, lockName types.NamespacedName, maxOwners int) *Checker {
	return &Checker{
		client:    client,
		lockName:  lockName,
		maxOwners: maxOwners,
	}
}

func (l *Checker) TryAcquireLock(ctx context.Context, owner metav1.Object) error {
	var lock corev1.ConfigMap
	if err := l.client.Get(ctx, l.lockName, &lock); err != nil {
		if apierrors.IsNotFound(err) {
			return l.createLock(ctx, owner)
		}
		return fmt.Errorf("failed to get lock: %w", err)
	}

	for _, ref := range lock.GetOwnerReferences() {
		if ref.Name == owner.GetName() && ref.UID == owner.GetUID() {
			return nil
		}
	}

	if l.maxOwners == 0 || len(lock.GetOwnerReferences()) < l.maxOwners {
		if err := controllerutil.SetOwnerReference(owner, &lock, l.client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}
		if err := l.client.Update(ctx, &lock); err != nil {
			return fmt.Errorf("failed to update lock: %w", err)
		}

		return nil
	}

	return ErrMaxPipelinesExceeded
}

func (l *Checker) IsLockHolder(ctx context.Context, obj metav1.Object) error {
	var lock corev1.ConfigMap
	if err := l.client.Get(ctx, l.lockName, &lock); err != nil {
		return &errortypes.APIRequestFailedError{
			Err: fmt.Errorf("failed to get lock: %w", err),
		}
	}

	for _, ref := range lock.GetOwnerReferences() {
		if ref.Name == obj.GetName() && ref.UID == obj.GetUID() {
			return nil
		}
	}

	return ErrMaxPipelinesExceeded
}

func (l *Checker) createLock(ctx context.Context, owner metav1.Object) error {
	lock := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      l.lockName.Name,
			Namespace: l.lockName.Namespace,
		},
	}

	if err := controllerutil.SetOwnerReference(owner, &lock, l.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	if err := l.client.Create(ctx, &lock); err != nil {
		return fmt.Errorf("failed to create lock: %w", err)
	}

	return nil
}
