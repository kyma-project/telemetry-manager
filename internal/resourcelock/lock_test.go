package resourcelock

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	l := NewLocker(fakeClient, lockName, 2)

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
	l := NewLocker(fakeClient, lockName, 2)

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

func TestTryAcquireLock_UnlimitedPipelines(t *testing.T) {
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
	// Create a locker with UnlimitedPipelineCount (-1) to allow unlimited pipelines
	l := NewLocker(fakeClient, lockName, UnlimitedPipelineCount)

	// All owners should be able to acquire the lock without any errors
	err := l.TryAcquireLock(ctx, owner1)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner2)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, owner3)
	require.NoError(t, err)

	// Verify that the lock ConfigMap was not created since unlimited pipelines bypass locking
	var lock corev1.ConfigMap

	err = fakeClient.Get(ctx, types.NamespacedName{Name: lockName.Name + "-lock", Namespace: lockName.Namespace}, &lock)
	require.Error(t, err, "Lock ConfigMap should not be created when unlimited pipelines is enabled")
}

func Test_new(t *testing.T) {
	type args struct {
		client    client.Client
		lockName  types.NamespacedName
		maxOwners int
		suffix    string
	}

	client := fake.NewClientBuilder().Build()

	tests := []struct {
		name string
		args args
		want *Checker
	}{
		{
			name: "Test newChecker with suffix",
			args: args{
				client: client,
				lockName: types.NamespacedName{
					Namespace: "test",
					Name:      "test-lock",
				},
				maxOwners: 2,
				suffix:    "lock",
			},
			want: &Checker{
				client:    client,
				lockName:  types.NamespacedName{Namespace: "test", Name: "test-lock"},
				maxOwners: 2,
			},
		},
		{
			name: "Test newChecker without suffix",
			args: args{
				client: client,
				lockName: types.NamespacedName{
					Namespace: "test",
					Name:      "test",
				},
				maxOwners: 2,
				suffix:    "lock",
			},
			want: &Checker{
				client:    client,
				lockName:  types.NamespacedName{Namespace: "test", Name: "test-lock"},
				maxOwners: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newChecker(tt.args.client, tt.args.lockName, tt.args.maxOwners, tt.args.suffix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newChecker() = %v, want %v", got, tt.want)
			}
		})
	}
}
