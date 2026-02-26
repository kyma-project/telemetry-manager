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

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
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
	clientset   kubernetes.Interface
	watchers    map[types.NamespacedName]*watcher
	eventRouter func(pipeline client.Object)
	stopped     bool
	mu          sync.RWMutex
	wg          sync.WaitGroup
}

// NewClient creates a new Client for watching Kubernetes secrets.
// It initializes the Kubernetes clientset using the provided REST configuration.
// Events are routed to the appropriate channel based on pipeline type:
// - TracePipeline events go to traceEventChan
// - MetricPipeline events go to metricEventChan
// - LogPipeline events go to logEventChan
func NewClient(cfg *rest.Config, traceEventChan, metricEventChan, logEventChan chan<- event.GenericEvent) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	eventRouter := func(pipeline client.Object) {
		ev := event.GenericEvent{Object: pipeline}

		switch pipeline.(type) {
		case *telemetryv1beta1.TracePipeline:
			traceEventChan <- ev
		case *telemetryv1beta1.MetricPipeline:
			metricEventChan <- ev
		case *telemetryv1beta1.LogPipeline:
			logEventChan <- ev
		default:
			logf.Log.Error(nil, "Unknown pipeline type, cannot route event", "pipelineType", fmt.Sprintf("%T", pipeline))
		}
	}

	return &Client{
		clientset:   clientset,
		watchers:    make(map[types.NamespacedName]*watcher),
		eventRouter: eventRouter,
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
	log := logf.FromContext(ctx).V(1)

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
			if w.link(pipeline) {
				log.Info("Linked pipeline to existing watcher",
					"secret", secret.String())
			}
		} else {
			w := newWatcher(secret, pipeline, c.clientset, c.eventRouter)
			c.startWatcher(ctx, w)
			c.watchers[secret] = w
			log.Info("Created new watcher for secret",
				"secret", secret.String())
		}
	}

	// Remove pipeline from watchers not in the current set
	for watchedSecret, w := range c.watchers {
		_, inCurrentSet := secretSet[watchedSecret]
		if !inCurrentSet && w.isLinked(pipeline) {
			// Remove this pipeline from the watcher's linked pipelines (thread-safe)
			hasPipelines := w.unlink(pipeline)
			log.Info("Unlinked pipeline from watcher",
				"secret", watchedSecret.String(),
				"watcherHasRemainingPipelines", hasPipelines)

			// If no pipelines are linked anymore, stop and delete the watcher
			if !hasPipelines {
				w.cancel()
				// Note: wg.Done() will be called by the watcher's goroutine defer
				delete(c.watchers, watchedSecret)
				log.Info("Stopped watcher with no remaining pipelines",
					"secret", watchedSecret.String())
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
	watcherCount := len(c.watchers)

	logf.Log.V(1).Info("Stopping secret watcher client",
		"watcherCount", watcherCount,
		"timeout", timeout)

	// Cancel all watchers
	for secret, entry := range c.watchers {
		if entry.cancel != nil {
			logf.Log.V(1).Info("Canceling watcher", "secret", secret.String())
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
	logf.FromContext(ctx).V(1).Info("Starting watcher goroutine", "secret", w.secret.String())

	watcherCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	c.wg.Go(func() {
		w.start(watcherCtx)
	})
}
