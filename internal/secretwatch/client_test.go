package secretwatch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestPipeline(name string) client.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}

func TestSecretWatchTriggersEvent(t *testing.T) {
	t.Run("should send event when watched secret is modified", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}
		clientset := fake.NewClientset(secret)

		// Create a fake watcher that we control
		fakeWatcher := watch.NewFake()
		clientset.PrependWatchReactor("secrets", clienttesting.DefaultWatchReactor(fakeWatcher, nil))

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		// Ensure cleanup: stop the fake watcher to unblock goroutines, then stop client
		t.Cleanup(func() {
			fakeWatcher.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline := newTestPipeline("my-pipeline")
		secretName := types.NamespacedName{Namespace: "default", Name: "my-secret"}

		// Start watching the secret
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})

		// Give the watcher goroutine time to start
		time.Sleep(50 * time.Millisecond)

		// Simulate secret modification via the fake watcher
		fakeWatcher.Modify(secret)

		// Wait for event
		select {
		case evt := <-eventChan:
			require.Equal(t, "my-pipeline", evt.Object.GetName())
		case <-time.After(2 * time.Second):
			t.Fatal("expected event was not received")
		}
	})

	t.Run("should send event when watched secret is added", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)
		clientset := fake.NewClientset()

		fakeWatcher := watch.NewFake()
		clientset.PrependWatchReactor("secrets", clienttesting.DefaultWatchReactor(fakeWatcher, nil))

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		t.Cleanup(func() {
			fakeWatcher.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline := newTestPipeline("my-pipeline")
		secretName := types.NamespacedName{Namespace: "default", Name: "my-secret"}

		// Start watching the secret (it doesn't exist yet)
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Simulate secret creation via the fake watcher
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-secret",
				Namespace: "default",
			},
		}
		fakeWatcher.Add(secret)

		select {
		case evt := <-eventChan:
			require.Equal(t, "my-pipeline", evt.Object.GetName())
		case <-time.After(2 * time.Second):
			t.Fatal("expected event was not received")
		}
	})

	t.Run("should send event to all linked pipelines", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-secret",
				Namespace: "default",
			},
		}
		clientset := fake.NewClientset(secret)

		fakeWatcher := watch.NewFake()
		clientset.PrependWatchReactor("secrets", clienttesting.DefaultWatchReactor(fakeWatcher, nil))

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		t.Cleanup(func() {
			fakeWatcher.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline1 := newTestPipeline("pipeline-1")
		pipeline2 := newTestPipeline("pipeline-2")
		secretName := types.NamespacedName{Namespace: "default", Name: "shared-secret"}

		// Both pipelines watch the same secret
		c.SyncWatchedSecrets(ctx, pipeline1, []types.NamespacedName{secretName})
		c.SyncWatchedSecrets(ctx, pipeline2, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Simulate secret modification
		fakeWatcher.Modify(secret)

		// Collect events
		receivedPipelines := make(map[string]bool)
		timeout := time.After(2 * time.Second)

		for len(receivedPipelines) < 2 {
			select {
			case evt := <-eventChan:
				receivedPipelines[evt.Object.GetName()] = true
			case <-timeout:
				t.Fatalf("expected 2 events, got %d", len(receivedPipelines))
			}
		}

		require.True(t, receivedPipelines["pipeline-1"])
		require.True(t, receivedPipelines["pipeline-2"])
	})

	t.Run("should send event when watched secret is deleted", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-secret",
				Namespace: "default",
			},
		}
		clientset := fake.NewClientset(secret)

		fakeWatcher := watch.NewFake()
		clientset.PrependWatchReactor("secrets", clienttesting.DefaultWatchReactor(fakeWatcher, nil))

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		t.Cleanup(func() {
			fakeWatcher.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline := newTestPipeline("my-pipeline")
		secretName := types.NamespacedName{Namespace: "default", Name: "my-secret"}

		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Simulate secret deletion
		fakeWatcher.Delete(secret)

		select {
		case evt := <-eventChan:
			require.Equal(t, "my-pipeline", evt.Object.GetName())
		case <-time.After(2 * time.Second):
			t.Fatal("expected event was not received")
		}
	})
}
