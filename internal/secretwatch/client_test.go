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

func TestSyncWatchedSecretsMultipleCalls(t *testing.T) {
	t.Run("should not trigger events for pipeline that no longer references a secret", func(t *testing.T) {
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

		pipelineA := newTestPipeline("pipeline-a")
		pipelineB := newTestPipeline("pipeline-b")
		secretName := types.NamespacedName{Namespace: "default", Name: "my-secret"}

		// Initially both pipelines watch the secret
		c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{secretName})
		c.SyncWatchedSecrets(ctx, pipelineB, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Pipeline A no longer references the secret
		c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{})

		// Drain any pending events from previous operations
		drainEvents(eventChan)

		// Simulate secret modification
		fakeWatcher.Modify(secret)

		// Collect events - should only receive event for pipeline-b
		receivedPipelines := make(map[string]bool)
		timeout := time.After(500 * time.Millisecond)

	collectLoop:
		for {
			select {
			case evt := <-eventChan:
				receivedPipelines[evt.Object.GetName()] = true
			case <-timeout:
				break collectLoop
			}
		}

		require.False(t, receivedPipelines["pipeline-a"], "pipeline-a should not receive events after unsubscribing")
		require.True(t, receivedPipelines["pipeline-b"], "pipeline-b should still receive events")
	})

	t.Run("should stop watcher when last pipeline unsubscribes", func(t *testing.T) {
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

		// Start watching
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Verify watcher exists
		c.mu.RLock()
		_, exists := c.watchers[secretName]
		c.mu.RUnlock()
		require.True(t, exists, "watcher should exist before unsubscribe")

		// Unsubscribe the only pipeline
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{})

		// Verify watcher was removed
		c.mu.RLock()
		_, exists = c.watchers[secretName]
		c.mu.RUnlock()
		require.False(t, exists, "watcher should be removed when last pipeline unsubscribes")
	})

	t.Run("should keep watcher when one of multiple pipelines unsubscribes", func(t *testing.T) {
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

		pipelineA := newTestPipeline("pipeline-a")
		pipelineB := newTestPipeline("pipeline-b")
		secretName := types.NamespacedName{Namespace: "default", Name: "my-secret"}

		// Both pipelines watch the secret
		c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{secretName})
		c.SyncWatchedSecrets(ctx, pipelineB, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Pipeline A unsubscribes
		c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{})

		// Verify watcher still exists (pipeline B is still subscribed)
		c.mu.RLock()
		w, exists := c.watchers[secretName]
		c.mu.RUnlock()
		require.True(t, exists, "watcher should still exist when one pipeline remains")
		require.False(t, w.isLinked(pipelineA), "pipeline-a should not be linked")
		require.True(t, w.isLinked(pipelineB), "pipeline-b should still be linked")
	})

	t.Run("should switch pipeline from one secret to another", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret1 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-1",
				Namespace: "default",
			},
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-2",
				Namespace: "default",
			},
		}
		clientset := fake.NewClientset(secret1, secret2)

		fakeWatcher1 := watch.NewFake()
		fakeWatcher2 := watch.NewFake()

		// Use field selector to route watches to the correct fake watcher
		clientset.PrependWatchReactor("secrets", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
			watchAction := action.(clienttesting.WatchAction)
			fieldSelector := watchAction.GetWatchRestrictions().Fields.String()
			if fieldSelector == "metadata.name=secret-1" {
				return true, fakeWatcher1, nil
			}
			if fieldSelector == "metadata.name=secret-2" {
				return true, fakeWatcher2, nil
			}
			return false, nil, nil
		})

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		t.Cleanup(func() {
			fakeWatcher1.Stop()
			fakeWatcher2.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline := newTestPipeline("my-pipeline")
		secretName1 := types.NamespacedName{Namespace: "default", Name: "secret-1"}
		secretName2 := types.NamespacedName{Namespace: "default", Name: "secret-2"}

		// Initially watch secret-1
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName1})

		time.Sleep(50 * time.Millisecond)

		// Verify watching secret-1
		c.mu.RLock()
		_, exists := c.watchers[secretName1]
		c.mu.RUnlock()
		require.True(t, exists, "should watch secret-1 initially")

		// Switch to watching secret-2 instead
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName2})

		time.Sleep(50 * time.Millisecond)

		// Verify no longer watching secret-1, now watching secret-2
		c.mu.RLock()
		_, exists1 := c.watchers[secretName1]
		_, exists2 := c.watchers[secretName2]
		c.mu.RUnlock()
		require.False(t, exists1, "should no longer watch secret-1")
		require.True(t, exists2, "should now watch secret-2")

		// Drain any pending events
		drainEvents(eventChan)

		// Modify secret-1 - should NOT trigger event (watcher was stopped)
		fakeWatcher1.Modify(secret1)

		// Modify secret-2 - should trigger event
		fakeWatcher2.Modify(secret2)

		// Should only receive event for secret-2 change
		receivedPipelines := make(map[string]int)
		timeout := time.After(500 * time.Millisecond)

	collectLoop:
		for {
			select {
			case evt := <-eventChan:
				receivedPipelines[evt.Object.GetName()]++
			case <-timeout:
				break collectLoop
			}
		}

		// Since secret-1 watcher was stopped, we should only get events from secret-2
		require.Equal(t, 1, receivedPipelines["my-pipeline"], "should receive exactly one event from secret-2")
	})

	t.Run("should handle pipeline watching multiple secrets then dropping one", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret1 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-1",
				Namespace: "default",
			},
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-2",
				Namespace: "default",
			},
		}
		clientset := fake.NewClientset(secret1, secret2)

		fakeWatcher1 := watch.NewFake()
		fakeWatcher2 := watch.NewFake()

		// Use field selector to route watches to the correct fake watcher
		clientset.PrependWatchReactor("secrets", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
			watchAction := action.(clienttesting.WatchAction)
			fieldSelector := watchAction.GetWatchRestrictions().Fields.String()
			if fieldSelector == "metadata.name=secret-1" {
				return true, fakeWatcher1, nil
			}
			if fieldSelector == "metadata.name=secret-2" {
				return true, fakeWatcher2, nil
			}
			return false, nil, nil
		})

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		t.Cleanup(func() {
			fakeWatcher1.Stop()
			fakeWatcher2.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline := newTestPipeline("my-pipeline")
		secretName1 := types.NamespacedName{Namespace: "default", Name: "secret-1"}
		secretName2 := types.NamespacedName{Namespace: "default", Name: "secret-2"}

		// Watch both secrets
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName1, secretName2})

		time.Sleep(50 * time.Millisecond)

		// Verify watching both secrets
		c.mu.RLock()
		_, exists1 := c.watchers[secretName1]
		_, exists2 := c.watchers[secretName2]
		c.mu.RUnlock()
		require.True(t, exists1, "should watch secret-1")
		require.True(t, exists2, "should watch secret-2")

		// Drain any pending events
		drainEvents(eventChan)

		// Now only watch secret-2 (drop secret-1)
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName2})

		time.Sleep(50 * time.Millisecond)

		// Verify secret-1 watcher is removed
		c.mu.RLock()
		_, exists1 = c.watchers[secretName1]
		_, exists2 = c.watchers[secretName2]
		c.mu.RUnlock()
		require.False(t, exists1, "should no longer watch secret-1")
		require.True(t, exists2, "should still watch secret-2")

		// Drain events from sync
		drainEvents(eventChan)

		// Modify secret-2 - should trigger event
		fakeWatcher2.Modify(secret2)

		select {
		case evt := <-eventChan:
			require.Equal(t, "my-pipeline", evt.Object.GetName())
		case <-time.After(2 * time.Second):
			t.Fatal("expected event was not received for secret-2")
		}
	})

	t.Run("should handle re-subscribing to a previously unwatched secret", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-secret",
				Namespace: "default",
			},
		}
		clientset := fake.NewClientset(secret)

		fakeWatcher1 := watch.NewFake()
		fakeWatcher2 := watch.NewFake()

		watcherCount := 0
		clientset.PrependWatchReactor("secrets", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
			watcherCount++
			if watcherCount == 1 {
				return true, fakeWatcher1, nil
			}
			return true, fakeWatcher2, nil
		})

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		t.Cleanup(func() {
			fakeWatcher1.Stop()
			fakeWatcher2.Stop()
			c.StopWithTimeout(100 * time.Millisecond)
		})

		pipeline := newTestPipeline("my-pipeline")
		secretName := types.NamespacedName{Namespace: "default", Name: "my-secret"}

		// Start watching
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})
		time.Sleep(50 * time.Millisecond)

		// Unsubscribe
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{})

		// Verify watcher removed
		c.mu.RLock()
		_, exists := c.watchers[secretName]
		c.mu.RUnlock()
		require.False(t, exists, "watcher should be removed after unsubscribe")

		// Re-subscribe
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})
		time.Sleep(50 * time.Millisecond)

		// Verify new watcher created
		c.mu.RLock()
		_, exists = c.watchers[secretName]
		c.mu.RUnlock()
		require.True(t, exists, "new watcher should be created after re-subscribe")

		// Drain events
		drainEvents(eventChan)

		// Modify secret via new watcher
		fakeWatcher2.Modify(secret)

		select {
		case evt := <-eventChan:
			require.Equal(t, "my-pipeline", evt.Object.GetName())
		case <-time.After(2 * time.Second):
			t.Fatal("expected event was not received after re-subscribing")
		}
	})

	t.Run("should be idempotent when called multiple times with same secrets", func(t *testing.T) {
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
		watcherCreateCount := 0
		clientset.PrependWatchReactor("secrets", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
			watcherCreateCount++
			return true, fakeWatcher, nil
		})

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

		// Call SyncWatchedSecrets multiple times with the same secrets
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})
		c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{secretName})

		time.Sleep(50 * time.Millisecond)

		// Verify only one watcher was created
		require.Equal(t, 1, watcherCreateCount, "should only create one watcher for repeated calls with same secret")

		// Verify watcher exists and pipeline is linked
		c.mu.RLock()
		w, exists := c.watchers[secretName]
		c.mu.RUnlock()
		require.True(t, exists)
		require.True(t, w.isLinked(pipeline))

		// Verify only one linked pipeline (not duplicated)
		linkedPipelines := w.getLinkedPipelines()
		require.Len(t, linkedPipelines, 1, "pipeline should not be linked multiple times")
	})
}

// drainEvents removes all pending events from the channel
func drainEvents(eventChan chan event.GenericEvent) {
	for {
		select {
		case <-eventChan:
		default:
			return
		}
	}
}
