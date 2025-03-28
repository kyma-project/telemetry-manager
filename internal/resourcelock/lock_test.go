package resourcelock

import (
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

	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	l := New(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, owner1)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner2)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner3)
	require.Equal(t, ErrMaxPipelinesExceeded, err)
}

func TestIsLockHolder(t *testing.T) {
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

	ctx := t.Context()
	fakeClient := fake.NewClientBuilder().Build()
	l := New(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, owner1)
	require.NoError(t, err)
	err = l.IsLockHolder(ctx, owner1)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner2)
	require.NoError(t, err)
	err = l.IsLockHolder(ctx, owner2)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner3)
	require.Equal(t, ErrMaxPipelinesExceeded, err)
	err = l.IsLockHolder(ctx, owner3)
	require.Equal(t, ErrMaxPipelinesExceeded, err)
}
