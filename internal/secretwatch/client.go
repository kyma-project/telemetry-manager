package secretwatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultShutdownTimeout = 30 * time.Second
)

var ErrClientStopped = errors.New("secret watcher client has been stopped")

// Client watches Kubernetes secrets and manages watchers for multiple pipelines.
// It tracks which pipelines depend on which secrets and maintains individual watchers
// for each unique secret being monitored.
// Client is safe for concurrent use.
type Client struct {
	clientset kubernetes.Interface
	watchers  map[types.NamespacedName]*watcher
	eventChan chan<- event.GenericEvent
	stopped   bool
	mu        sync.RWMutex
	wg        sync.WaitGroup
}

// NewClient creates a new Client for watching Kubernetes secrets.
// It initializes the Kubernetes clientset using the provided REST configuration.
// The eventHandler will be called whenever a watched secret changes.
// If eventHandler is nil, events will only be logged.
func NewClient(cfg *rest.Config, eventChan chan<- event.GenericEvent) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		watchers:  make(map[types.NamespacedName]*watcher),
		eventChan: eventChan,
	}, nil
}

// SyncWatchedSecrets ensures the pipeline watches exactly the given set of secrets.
// It adds watchers for new secrets, removes the pipeline from secrets no longer needed,
// and cleans up watchers that have no remaining linked pipelines.
// This method is idempotent and declarative - it synchronizes to the desired state.
// New watchers are started immediately, and removed watchers are stopped immediately.
// Duplicate secrets in the input slice are automatically deduplicated.
// Returns ErrClientStopped if the client has been stopped.
func (c *Client) SyncWatchedSecrets(ctx context.Context, pipeline client.Object, secrets []types.NamespacedName) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return ErrClientStopped
	}

	// Deduplicate secrets using a set
	secretSet := make(map[types.NamespacedName]struct{}, len(secrets))
	for _, s := range secrets {
		secretSet[s] = struct{}{}
	}

	// Add or update watchers for the given secrets
	for secret := range secretSet {
		if w, exists := c.watchers[secret]; exists {
			// Watcher exists, link pipeline if not already linked (thread-safe)
			w.link(pipeline)
		} else {
			// Create new watcher and start it immediately
			w := newWatcher(secret, pipeline, c.clientset, c.eventChan)
			c.startWatcher(ctx, w)
			c.watchers[secret] = w
		}
	}

	// Remove pipeline from watchers not in the current set
	for watchedSecret, w := range c.watchers {
		_, secretFound := secretSet[watchedSecret]
		if !secretFound && w.isLinked(pipeline) {
			// Remove this pipeline from the watcher's linked pipelines (thread-safe)
			hasPipelines := w.unlink(pipeline)

			// If no pipelines are linked anymore, stop and delete the watcher
			if !hasPipelines {
				w.cancel()
				// Note: wg.Done() will be called by the watcher's goroutine defer
				delete(c.watchers, watchedSecret)
			}
		}
	}

	return nil
}

// Stop gracefully shuts down all watchers with the default timeout.
// It cancels all watcher contexts and waits for them to finish.
// If watchers don't finish within the default timeout, it returns without waiting further.
// After calling Stop, the Client cannot be reused. Calls to SyncWatchedSecrets will return ErrClientStopped.
func (c *Client) Stop() {
	c.stopWithTimeout(defaultShutdownTimeout)
}

func (c *Client) stopWithTimeout(timeout time.Duration) {
	c.mu.Lock()

	c.stopped = true

	// Cancel all watchers
	for _, entry := range c.watchers {
		if entry.cancel != nil {
			entry.cancel()
		}
	}

	// Unlock before waiting so that concurrent SyncWatchedSecrets calls
	// can return ErrClientStopped immediately instead of blocking.
	c.mu.Unlock()

	// Wait for all watchers to finish with timeout
	done := make(chan struct{})

	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logf.Log.V(1).Info("All secret watchers stopped gracefully")
	case <-time.After(timeout):
		logf.Log.Info("Secret watcher shutdown timeout exceeded, some watchers may still be running",
			"timeout", timeout)
	}
}

//nolint:contextcheck // Intentionally using Background() so watcher outlives reconcile request
func (c *Client) startWatcher(ctx context.Context, w *watcher) {
	watcherCtx, cancel := context.WithCancel(
		logf.IntoContext(context.Background(), logf.FromContext(ctx)),
	)
	w.cancel = cancel

	c.wg.Go(func() {
		w.start(watcherCtx)
	})
}
