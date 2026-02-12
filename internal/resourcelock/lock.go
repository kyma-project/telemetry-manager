package resourcelock

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/errortypes"
)

var ErrMaxPipelinesExceeded = errors.New("maximum pipeline count limit exceeded")

const (
	MaxPipelineCount       = 5
	UnlimitedPipelineCount = -1
)

type Checker struct {
	client    client.Client
	lockName  types.NamespacedName
	maxOwners int
}

func newChecker(client client.Client, lockName types.NamespacedName, maxOwners int, suffix string) *Checker {
	if !strings.HasSuffix(lockName.Name, "-"+suffix) {
		lockName.Name = fmt.Sprintf("%s-%s", lockName.Name, suffix)
	}

	return &Checker{
		client:    client,
		lockName:  lockName,
		maxOwners: maxOwners,
	}
}

func NewLocker(client client.Client, lockName types.NamespacedName, maxOwners int) *Checker {
	return newChecker(client, lockName, maxOwners, "lock")
}

func NewSyncer(client client.Client, lockName types.NamespacedName) *Checker {
	return newChecker(client, lockName, 0, "syncer")
}

func (c *Checker) TryAcquireLock(ctx context.Context, owner metav1.Object) error {
	if c.maxOwners == UnlimitedPipelineCount {
		logf.FromContext(ctx, "Unlimited Pipeline count configured, skipping lock acquisition")
		return nil
	}

	var lock corev1.ConfigMap
	if err := c.client.Get(ctx, c.lockName, &lock); err != nil {
		if apierrors.IsNotFound(err) {
			return c.createLock(ctx, owner)
		}

		return fmt.Errorf("failed to get lock: %w", err)
	}

	for _, ref := range lock.GetOwnerReferences() {
		if ref.Name == owner.GetName() && ref.UID == owner.GetUID() {
			return nil
		}
	}

	if c.maxOwners == 0 || len(lock.GetOwnerReferences()) < c.maxOwners {
		if err := controllerutil.SetOwnerReference(owner, &lock, c.client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		if err := c.client.Update(ctx, &lock); err != nil {
			return fmt.Errorf("failed to update lock: %w", err)
		}

		return nil
	}

	return ErrMaxPipelinesExceeded
}

func (c *Checker) IsLockHolder(ctx context.Context, obj metav1.Object) error {
	var lock corev1.ConfigMap
	if err := c.client.Get(ctx, c.lockName, &lock); err != nil {
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

func (c *Checker) createLock(ctx context.Context, owner metav1.Object) error {
	lock := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.lockName.Name,
			Namespace: c.lockName.Namespace,
		},
	}

	if err := controllerutil.SetOwnerReference(owner, &lock, c.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	if err := c.client.Create(ctx, &lock); err != nil {
		return fmt.Errorf("failed to create lock: %w", err)
	}

	return nil
}
