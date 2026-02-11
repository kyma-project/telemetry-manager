package secretwatch

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 30 * time.Second
)

// EventType represents the type of secret event
type EventType = watch.EventType

// EventHandler is called when a watched secret changes.
// It receives the secret name, event type, and list of pipelines that depend on this secret.
type EventHandler func(secretName types.NamespacedName, eventType EventType, linkedPipelines []string)

type watcherEntry struct {
	watcher *watcher
	cancel  context.CancelFunc
	ctx     context.Context
}

// Client watches Kubernetes secrets and manages watchers for multiple pipelines.
// It tracks which pipelines depend on which secrets and maintains individual watchers
// for each unique secret being monitored.
// Client is safe for concurrent use.
type Client struct {
	clientset    kubernetes.Interface
	watchers     map[types.NamespacedName]*watcherEntry
	mu           sync.RWMutex
	eventHandler EventHandler
	wg           sync.WaitGroup
}

// NewClient creates a new Client for watching Kubernetes secrets.
// It initializes the Kubernetes clientset using the provided REST configuration.
// The eventHandler will be called whenever a watched secret changes.
// If eventHandler is nil, events will only be logged.
func NewClient(cfg *rest.Config, eventHandler EventHandler) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset:    clientset,
		watchers:     make(map[types.NamespacedName]*watcherEntry),
		eventHandler: eventHandler,
	}, nil
}

// Stop gracefully shuts down all watchers with the default timeout.
// It cancels all watcher contexts and waits for them to finish.
// If watchers don't finish within the default timeout, it returns without waiting further.
// After calling Stop, the Client can be restarted by calling Start again.
func (c *Client) Stop() {
	c.StopWithTimeout(DefaultShutdownTimeout)
}

// StopWithTimeout gracefully shuts down all watchers with a custom timeout.
// It cancels all watcher contexts and waits for them to finish.
// If watchers don't finish within the timeout, it logs a warning and returns.
// After calling StopWithTimeout, the Client can be restarted by calling Start again.
func (c *Client) StopWithTimeout(timeout time.Duration) {
	c.mu.Lock()

	// Cancel all watchers
	for _, entry := range c.watchers {
		if entry.cancel != nil {
			entry.cancel()
		}
	}

	c.mu.Unlock()

	// Wait for all watchers to finish with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All watchers finished gracefully
		logf.Log.V(1).Info("All secret watchers stopped gracefully")
	case <-time.After(timeout):
		// Timeout exceeded
		logf.Log.Info("Secret watcher shutdown timeout exceeded, some watchers may still be running",
			"timeout", timeout)
	}
}

// SyncWatchedSecrets ensures the pipeline watches exactly the given set of secrets.
// It adds watchers for new secrets, removes the pipeline from secrets no longer needed,
// and cleans up watchers that have no remaining linked pipelines.
// This method is idempotent and declarative - it synchronizes to the desired state.
// New watchers are started immediately, and removed watchers are stopped immediately.
func (c *Client) SyncWatchedSecrets(ctx context.Context, pipelineName string, secrets []types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add or update watchers for the given secrets
	for _, secret := range secrets {
		if entry, exists := c.watchers[secret]; exists {
			// Watcher exists, add pipeline if not already linked (thread-safe)
			entry.watcher.addPipeline(pipelineName)
		} else {
			// Create new watcher and start it immediately
			w := newWatcher(secret, []string{pipelineName}, c.clientset, c.eventHandler)

			watcherCtx, cancel := context.WithCancel(ctx)
			entry := &watcherEntry{
				watcher: w,
				cancel:  cancel,
				ctx:     watcherCtx,
			}

			c.wg.Add(1)
			go func(watcher *watcher) {
				defer c.wg.Done()
				watcher.Start(watcherCtx)
			}(w)

			c.watchers[secret] = entry
		}
	}

	// Remove pipeline from watchers not in the current set
	for watchedSecret, entry := range c.watchers {
		secretFound := slices.Contains(secrets, watchedSecret)
		if !secretFound && entry.watcher.hasPipeline(pipelineName) {
			// Remove this pipeline from the watcher's linked pipelines (thread-safe)
			hasPipelines := entry.watcher.removePipeline(pipelineName)

			// If no pipelines are linked anymore, stop and delete the watcher
			if !hasPipelines {
				if entry.cancel != nil {
					entry.cancel()
					// Note: wg.Done() will be called by the watcher's goroutine defer
				}
				delete(c.watchers, watchedSecret)
			}
		}
	}
}

// RemovePipeline removes all secret watches for the specified pipeline.
// This is a convenience method equivalent to calling SyncWatchedSecrets with an empty secret list.
func (c *Client) RemovePipeline(ctx context.Context, pipelineName string) {
	c.SyncWatchedSecrets(ctx, pipelineName, nil)
}
