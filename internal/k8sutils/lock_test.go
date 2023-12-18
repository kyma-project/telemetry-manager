package k8sutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	lockName = types.NamespacedName{
		Name:      "lock",
		Namespace: "default",
	}
)

func TestTryAcquireLock(t *testing.T) {
	owner1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner1",
			Namespace: "default",
		},
	}
	owner2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner2",
			Namespace: "default",
		},
	}
	owner3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner3",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	l := NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, owner1)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner2)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner3)
	require.Equal(t, errLockInUse, err)
}

func TestIsLockHolder(t *testing.T) {
	owner1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner1",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	l := NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, owner1)
	require.NoError(t, err)

	isLockHolder, err := l.IsLockHolder(ctx, owner1)
	require.NoError(t, err)
	require.True(t, isLockHolder)
}

func TestIsNotLockHolder(t *testing.T) {
	owner1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner1",
			Namespace: "default",
		},
	}
	owner2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner2",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	l := NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, owner1)
	require.NoError(t, err)

	isLockHolder, err := l.IsLockHolder(ctx, owner2)
	require.NoError(t, err)
	require.False(t, isLockHolder)
}
