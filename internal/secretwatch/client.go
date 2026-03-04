package secretwatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
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

// SyncWatchers ensures the pipeline watches exactly the given set of secrets.
// It adds watchers for new secrets, removes the pipeline from secrets no longer needed,
// and cleans up watchers that have no remaining linked pipelines.
// This method is idempotent and declarative - it synchronizes to the desired state.
// New watchers are started immediately, and removed watchers are stopped immediately.
// Duplicate secrets in the input slice are automatically deduplicated.
// Returns ErrClientStopped if the client has been stopped.
func (c *Client) SyncWatchers(ctx context.Context, pipeline client.Object, secrets []types.NamespacedName) error {
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
	pipelineKind := pipeline.GetObjectKind().GroupVersionKind().Kind

	for secret := range secretSet {
		if w, exists := c.watchers[secret]; exists {
			if w.link(pipeline) {
				logf.FromContext(ctx).V(1).Info("Linked pipeline to existing watcher",
					"secret", secret.String())
				metrics.SecretWatchersLinkedPipelines.WithLabelValues(secret.Namespace, secret.Name, pipelineKind).Inc()
			}
		} else {
			w := newWatcher(secret, pipeline, c.clientset, c.eventRouter)
			c.startWatcher(ctx, w)
			c.watchers[secret] = w
			logf.FromContext(ctx).V(1).Info("Created new watcher for secret",
				"secret", secret.String())
			metrics.SecretWatchersActive.WithLabelValues(secret.Namespace, secret.Name).Inc()
			metrics.SecretWatchersLinkedPipelines.WithLabelValues(secret.Namespace, secret.Name, pipelineKind).Inc()
		}
	}

	// Remove pipeline from watchers not in the current set
	c.unlinkPipelineFromWatchers(ctx, pipeline.GetName(), pipeline.GetObjectKind().GroupVersionKind(), secretSet)

	return nil
}

// RemoveFromWatchers removes a pipeline from all watchers by name and GVK.
// This should be called when a pipeline is deleted to clean up any lingering watchers.
// Watchers that have no remaining linked pipelines after removal are stopped and deleted.
// Returns ErrClientStopped if the client has been stopped.
func (c *Client) RemoveFromWatchers(ctx context.Context, name string, gvk schema.GroupVersionKind) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return ErrClientStopped
	}

	c.unlinkPipelineFromWatchers(ctx, name, gvk, nil)

	return nil
}

// Stop gracefully shuts down all watchers with the default timeout.
// It cancels all watcher contexts and waits for them to finish.
// If watchers don't finish within the default timeout, it returns without waiting further.
// After calling Stop, the Client cannot be reused. Calls to SyncWatchers will return ErrClientStopped.
func (c *Client) Stop(ctx context.Context) {
	c.stopWithTimeout(ctx, defaultShutdownTimeout)
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

// unlinkPipelineFromWatchers removes a pipeline from watchers not in the excludeSet.
// If excludeSet is nil, the pipeline is removed from all watchers.
// Watchers with no remaining linked pipelines are stopped and deleted.
func (c *Client) unlinkPipelineFromWatchers(ctx context.Context, name string, gvk schema.GroupVersionKind, excludeSet map[types.NamespacedName]struct{}) {
	pipelineKind := gvk.Kind

	for watchedSecret, w := range c.watchers {
		if excludeSet != nil {
			if _, inExcludeSet := excludeSet[watchedSecret]; inExcludeSet {
				continue
			}
		}

		if !w.isLinked(name, gvk) {
			continue
		}

		hasPipelines := w.unlink(name, gvk)
		logf.FromContext(ctx).V(1).Info("Unlinked pipeline from watcher",
			"secret", watchedSecret.String(),
			"watcherHasRemainingPipelines", hasPipelines)
		metrics.SecretWatchersLinkedPipelines.WithLabelValues(watchedSecret.Namespace, watchedSecret.Name, pipelineKind).Dec()

		// If no pipelines are linked anymore, stop and delete the watcher
		if !hasPipelines {
			w.cancel()
			delete(c.watchers, watchedSecret)
			logf.FromContext(ctx).V(1).Info("Stopped watcher with no remaining pipelines",
				"secret", watchedSecret.String())
			metrics.SecretWatchersActive.WithLabelValues(watchedSecret.Namespace, watchedSecret.Name).Set(0)
		}
	}
}

func (c *Client) stopWithTimeout(ctx context.Context, timeout time.Duration) {
	c.mu.Lock()

	c.stopped = true
	watcherCount := len(c.watchers)

	logf.FromContext(ctx).V(1).Info("Stopping secret watcher client",
		"watcherCount", watcherCount,
		"timeout", timeout)

	// Cancel all watchers
	for secret, entry := range c.watchers {
		if entry.cancel != nil {
			logf.FromContext(ctx).V(1).Info("Canceling watcher", "secret", secret.String())
			entry.cancel()
		}
	}

	// Unlock before waiting so that concurrent SyncWatchers calls
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
		logf.FromContext(ctx).V(1).Info("All secret watchers stopped gracefully")
	case <-time.After(timeout):
		logf.FromContext(ctx).Info("Secret watcher shutdown timeout exceeded, some watchers may still be running",
			"timeout", timeout)
	}
}
