package pipelinesyncermigration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const testNamespace = "kyma-system"

func TestDeleteOldSyncersIfNeeded(t *testing.T) {
	tests := []struct {
		name          string
		existingNames []string
		expectDeleted []string
	}{
		{
			name:          "no legacy ConfigMaps present",
			existingNames: []string{},
			expectDeleted: []string{},
		},
		{
			name:          "all legacy ConfigMaps present",
			existingNames: legacySyncerNames,
			expectDeleted: legacySyncerNames,
		},
		{
			name:          "only one legacy ConfigMap present",
			existingNames: []string{"telemetry-metricpipeline-sync-syncer"},
			expectDeleted: []string{"telemetry-metricpipeline-sync-syncer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme(t)

			var objects []client.Object
			for _, name := range tt.existingNames {
				objects = append(objects, newConfigMap(name))
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			migrator := New(fakeClient, logr.Discard(), testNamespace)

			err := migrator.deleteOldSyncersIfNeeded(context.Background())
			require.NoError(t, err)

			for _, name := range legacySyncerNames {
				var cm corev1.ConfigMap
				getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNamespace}, &cm)
				require.True(t, apierrors.IsNotFound(getErr), "expected %q to be deleted or absent", name)
			}
		})
	}
}

func TestDeleteOldSyncersIfNeeded_Idempotent(t *testing.T) {
	scheme := newTestScheme(t)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	migrator := New(fakeClient, logr.Discard(), testNamespace)

	require.NoError(t, migrator.deleteOldSyncersIfNeeded(context.Background()))
	require.NoError(t, migrator.deleteOldSyncersIfNeeded(context.Background()))
}

func TestDeleteOldSyncersIfNeeded_GetError(t *testing.T) {
	fakeClient := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return errors.New("api error")
			},
		}).
		Build()

	migrator := New(fakeClient, logr.Discard(), testNamespace)

	require.Error(t, migrator.deleteOldSyncersIfNeeded(context.Background()))
}

func TestStart_RetriesUntilContextCancelled(t *testing.T) {
	fakeClient := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(newConfigMap("telemetry-logpipeline-sync-syncer")).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return errors.New("transient error")
			},
		}).
		Build()

	migrator := New(fakeClient, logr.Discard(), testNamespace)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	require.NoError(t, migrator.Start(ctx))
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	return scheme
}

func newConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
}
