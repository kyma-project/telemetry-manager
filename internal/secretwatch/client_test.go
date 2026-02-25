package secretwatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/event"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

const (
	testEventTimeout    = 2 * time.Second
	testShutdownTimeout = 100 * time.Millisecond
	testStartupDelay    = 50 * time.Millisecond
	testNamespace       = "default"
	testSecretName1     = "secret-1"
	testSecretName2     = "secret-2"
)

var (
	testSecret1 = types.NamespacedName{Namespace: testNamespace, Name: testSecretName1}
	testSecret2 = types.NamespacedName{Namespace: testNamespace, Name: testSecretName2}
)

func TestSecretWatchTriggersEvent(t *testing.T) {
	t.Run("should send event when watched secret is modified", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Start watching the secret
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))

		// Give the watcher goroutine time to start
		time.Sleep(testStartupDelay)

		// Simulate secret modification via the fake watcher
		fakeWatcher.Modify(secret)

		// Wait for event
		received := collectEvents(eventChan)
		require.Contains(t, received, pipeline.GetName())
		require.Equal(t, 1, received[pipeline.GetName()], "expected exactly one event")
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Start watching the secret (it doesn't exist yet)
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Simulate secret creation via the fake watcher
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
			},
		}
		fakeWatcher.Add(secret)

		received := collectEvents(eventChan)
		require.Contains(t, received, pipeline.GetName())
		require.Equal(t, 1, received[pipeline.GetName()], "expected exactly one event")
	})

	t.Run("should send event to all linked pipelines", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline1 := new(testutils.NewLogPipelineBuilder().WithName("pipeline-1").Build())
		pipeline2 := new(testutils.NewLogPipelineBuilder().WithName("pipeline-2").Build())

		// Both pipelines watch the same secret
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline1, []types.NamespacedName{testSecret1}))
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline2, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Simulate secret modification
		fakeWatcher.Modify(secret)

		// Collect events
		receivedPipelines := make(map[string]bool)
		timeout := time.After(testEventTimeout)

		for len(receivedPipelines) < 2 {
			select {
			case evt := <-eventChan:
				receivedPipelines[evt.Object.GetName()] = true
			case <-timeout:
				require.FailNow(t, "expected 2 events", "got %d", len(receivedPipelines))
			}
		}

		require.Contains(t, receivedPipelines, pipeline1.GetName())
		require.Contains(t, receivedPipelines, pipeline2.GetName())
	})

	t.Run("should send event when watched secret is deleted", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Simulate secret deletion
		fakeWatcher.Delete(secret)

		received := collectEvents(eventChan)
		require.Contains(t, received, pipeline.GetName())
		require.Equal(t, 1, received[pipeline.GetName()], "expected exactly one event")
	})
}

//nolint:gocognit // High complexity due to multiple table-driven subtests
func TestSyncWatchedSecretsMultipleCalls(t *testing.T) {
	t.Run("should not trigger events for pipeline that no longer references a secret", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipelineA := new(testutils.NewLogPipelineBuilder().WithName("pipeline-a").Build())
		pipelineB := new(testutils.NewLogPipelineBuilder().WithName("pipeline-b").Build())

		// Initially both pipelines watch the secret
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{testSecret1}))
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipelineB, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Pipeline A no longer references the secret
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{}))

		// Drain any pending events from previous operations
		drainEvents(eventChan)

		// Simulate secret modification
		fakeWatcher.Modify(secret)

		// Collect events - should only receive event for pipeline-b
		receivedPipelines := collectEvents(eventChan)

		require.NotContains(t, receivedPipelines, pipelineA.GetName(), "pipeline-a should not receive events after unsubscribing")
		require.Contains(t, receivedPipelines, pipelineB.GetName(), "pipeline-b should still receive events")
	})

	t.Run("should stop watcher when last pipeline unsubscribes", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Start watching
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Verify watcher exists
		c.mu.RLock()
		_, exists := c.watchers[testSecret1]
		c.mu.RUnlock()
		require.True(t, exists, "watcher should exist before unsubscribe")

		// Unsubscribe the only pipeline
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{}))

		// Verify watcher was removed
		c.mu.RLock()
		_, exists = c.watchers[testSecret1]
		c.mu.RUnlock()
		require.False(t, exists, "watcher should be removed when last pipeline unsubscribes")
	})

	t.Run("should keep watcher when one of multiple pipelines unsubscribes", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipelineA := new(testutils.NewLogPipelineBuilder().WithName("pipeline-a").Build())
		pipelineB := new(testutils.NewLogPipelineBuilder().WithName("pipeline-b").Build())

		// Both pipelines watch the secret
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{testSecret1}))
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipelineB, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Pipeline A unsubscribes
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipelineA, []types.NamespacedName{}))

		// Verify watcher still exists (pipeline B is still subscribed)
		c.mu.RLock()
		w, exists := c.watchers[testSecret1]
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
				Name:      testSecretName1,
				Namespace: testNamespace,
			},
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName2,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Initially watch secret-1
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Verify watching secret-1
		c.mu.RLock()
		_, exists := c.watchers[testSecret1]
		c.mu.RUnlock()
		require.True(t, exists, "should watch secret-1 initially")

		// Switch to watching secret-2 instead
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret2}))

		time.Sleep(testStartupDelay)

		// Verify no longer watching secret-1, now watching secret-2
		c.mu.RLock()
		_, exists1 := c.watchers[testSecret1]
		_, exists2 := c.watchers[testSecret2]
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
		receivedPipelines := collectEvents(eventChan)

		// Since secret-1 watcher was stopped, we should only get events from secret-2
		require.Contains(t, receivedPipelines, pipeline.GetName())
		require.Equal(t, 1, receivedPipelines[pipeline.GetName()], "expected exactly one event")
	})

	t.Run("should handle pipeline watching multiple secrets then dropping one", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret1 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
			},
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName2,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Watch both secrets
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1, testSecret2}))

		time.Sleep(testStartupDelay)

		// Verify watching both secrets
		c.mu.RLock()
		_, exists1 := c.watchers[testSecret1]
		_, exists2 := c.watchers[testSecret2]
		c.mu.RUnlock()
		require.True(t, exists1, "should watch secret-1")
		require.True(t, exists2, "should watch secret-2")

		// Drain any pending events
		drainEvents(eventChan)

		// Now only watch secret-2 (drop secret-1)
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret2}))

		time.Sleep(testStartupDelay)

		// Verify secret-1 watcher is removed
		c.mu.RLock()
		_, exists1 = c.watchers[testSecret1]
		_, exists2 = c.watchers[testSecret2]
		c.mu.RUnlock()
		require.False(t, exists1, "should no longer watch secret-1")
		require.True(t, exists2, "should still watch secret-2")

		// Drain events from sync
		drainEvents(eventChan)

		// Modify secret-2 - should trigger event
		fakeWatcher2.Modify(secret2)

		received := collectEvents(eventChan)
		require.Contains(t, received, pipeline.GetName())
		require.Equal(t, 1, received[pipeline.GetName()], "expected exactly one event")
	})

	t.Run("should handle re-subscribing to a previously unwatched secret", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Start watching
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))
		time.Sleep(testStartupDelay)

		// Unsubscribe
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{}))

		// Verify watcher removed
		c.mu.RLock()
		_, exists := c.watchers[testSecret1]
		c.mu.RUnlock()
		require.False(t, exists, "watcher should be removed after unsubscribe")

		// Re-subscribe
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))
		time.Sleep(testStartupDelay)

		// Verify new watcher created
		c.mu.RLock()
		_, exists = c.watchers[testSecret1]
		c.mu.RUnlock()
		require.True(t, exists, "new watcher should be created after re-subscribe")

		// Drain events
		drainEvents(eventChan)

		// Modify secret via new watcher
		fakeWatcher2.Modify(secret)

		received := collectEvents(eventChan)
		require.Contains(t, received, pipeline.GetName())
		require.Equal(t, 1, received[pipeline.GetName()], "expected exactly one event")
	})

	t.Run("should be idempotent when called multiple times with same secrets", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := new(testutils.NewLogPipelineBuilder().WithName("my-pipeline").Build())

		// Call SyncWatchedSecrets multiple times with the same secrets
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))
		require.NoError(t, c.SyncWatchedSecrets(ctx, pipeline, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Verify only one watcher was created
		require.Equal(t, 1, watcherCreateCount, "should only create one watcher for repeated calls with same secret")

		// Verify watcher exists and pipeline is linked
		c.mu.RLock()
		w, exists := c.watchers[testSecret1]
		c.mu.RUnlock()
		require.True(t, exists)
		require.True(t, w.isLinked(pipeline))

		// Verify only one linked pipeline (not duplicated)
		linkedPipelines := w.getLinkedPipelines()
		require.Len(t, linkedPipelines, 1, "pipeline should not be linked multiple times")
	})
}

func TestSyncWatchedSecretsAfterStop(t *testing.T) {
	t.Run("should return error when called after Stop", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)
		clientset := fake.NewClientset()

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		pipeline := testutils.NewLogPipelineBuilder().Build()

		// Stop the client
		c.stopWithTimeout(testShutdownTimeout)

		// Try to sync secrets after stop
		err := c.SyncWatchedSecrets(ctx, &pipeline, []types.NamespacedName{testSecret1})

		require.ErrorIs(t, err, ErrClientStopped)
	})

	t.Run("should return error when called after Stop using public method", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)
		clientset := fake.NewClientset()

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		pipeline := testutils.NewLogPipelineBuilder().Build()

		// Stop the client using the public method
		c.Stop()

		// Try to sync secrets after stop
		err := c.SyncWatchedSecrets(ctx, &pipeline, []types.NamespacedName{testSecret1})

		require.ErrorIs(t, err, ErrClientStopped)
	})
}

func TestWatcherErrorHandling(t *testing.T) {
	t.Run("should handle watch error event and continue", func(t *testing.T) {
		ctx := context.Background()
		eventChan := make(chan event.GenericEvent, 10)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName1,
				Namespace: testNamespace,
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
			c.stopWithTimeout(testShutdownTimeout)
		})

		pipeline := testutils.NewLogPipelineBuilder().Build()

		require.NoError(t, c.SyncWatchedSecrets(ctx, &pipeline, []types.NamespacedName{testSecret1}))

		time.Sleep(testStartupDelay)

		// Send a watch error event
		fakeWatcher.Error(&metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "test error",
		})

		// Give time for the error to be processed
		time.Sleep(testStartupDelay)

		// The watcher should still exist (it reconnects on error)
		c.mu.RLock()
		_, exists := c.watchers[testSecret1]
		c.mu.RUnlock()
		require.True(t, exists, "watcher should still exist after error")
	})

	t.Run("should handle context cancellation during reconnect delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		eventChan := make(chan event.GenericEvent, 10)

		clientset := fake.NewClientset()

		// Return an error on watch to trigger reconnect logic
		clientset.PrependWatchReactor("secrets", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
			return true, nil, errors.New("simulated watch error")
		})

		c := &Client{
			clientset: clientset,
			watchers:  make(map[types.NamespacedName]*watcher),
			eventChan: eventChan,
		}

		pipeline := testutils.NewLogPipelineBuilder().Build()

		require.NoError(t, c.SyncWatchedSecrets(ctx, &pipeline, []types.NamespacedName{testSecret1}))

		// Give time for the watcher to hit the error and enter reconnect delay
		time.Sleep(testStartupDelay)

		// Cancel the context during the reconnect delay
		cancel()

		// Give time for the watcher to notice the cancellation
		time.Sleep(testStartupDelay)

		// Stop client to clean up
		c.stopWithTimeout(testShutdownTimeout)
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

// collectEvents collects events from the channel until timeout and returns a map of pipeline names to event counts
func collectEvents(eventChan chan event.GenericEvent) map[string]int {
	received := make(map[string]int)
	timer := time.After(testEventTimeout)

	for {
		select {
		case evt := <-eventChan:
			received[evt.Object.GetName()]++
		case <-timer:
			return received
		}
	}
}
