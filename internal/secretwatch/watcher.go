package secretwatch

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	kubecoreev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/metrics"
)

const reconnectDelay = 5 * time.Second

// watcher monitors a single Kubernetes secret and tracks which pipelines depend on it.
// It automatically reconnects on errors and handles resource version updates.
type watcher struct {
	secret      types.NamespacedName
	linked      []client.Object
	client      kubecoreev1.SecretInterface
	eventRouter func(pipeline client.Object)
	mu          sync.RWMutex
	cancel      context.CancelFunc
}

// newWatcher creates a new watcher for the specified secret with the initial pipeline.
// Call start() to begin watching in a background goroutine.
func newWatcher(
	secret types.NamespacedName,
	pipeline client.Object,
	clientset kubernetes.Interface,
	eventRouter func(pipeline client.Object),
) *watcher {
	return &watcher{
		secret:      secret,
		linked:      []client.Object{pipeline},
		client:      clientset.CoreV1().Secrets(secret.Namespace),
		eventRouter: eventRouter,
	}
}

// samePipeline checks if two pipelines are the same by comparing both name and GVK.
// This is necessary because different pipeline types (LogPipeline, MetricPipeline, TracePipeline)
// can have the same name but are distinct objects.
func samePipeline(a, b client.Object) bool {
	return a.GetName() == b.GetName() &&
		a.GetObjectKind().GroupVersionKind() == b.GetObjectKind().GroupVersionKind()
}

// link adds a pipeline to the watcher's linked pipelines if it's not already linked.
// It returns true if the pipeline was added, or false if it was already linked.
func (w *watcher) link(pipeline client.Object) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if slices.ContainsFunc(w.linked, func(p client.Object) bool {
		return samePipeline(p, pipeline)
	}) {
		return false
	}

	w.linked = append(w.linked, pipeline)

	return true
}

func (w *watcher) isLinked(name string, gvk schema.GroupVersionKind) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return slices.ContainsFunc(w.linked, func(p client.Object) bool {
		return p.GetName() == name && p.GetObjectKind().GroupVersionKind() == gvk
	})
}

// unlink removes a pipeline from the watcher's linked pipelines by name and GVK.
// It returns true if there are still pipelines linked after removal, or false if the list is now empty.
func (w *watcher) unlink(name string, gvk schema.GroupVersionKind) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.linked = slices.DeleteFunc(w.linked, func(p client.Object) bool {
		return p.GetName() == name && p.GetObjectKind().GroupVersionKind() == gvk
	})

	return len(w.linked) > 0
}

func (w *watcher) getLinkedPipelines() []client.Object {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return slices.Clone(w.linked)
}

func (w *watcher) secretNameFieldSelector() string {
	return fmt.Sprintf("metadata.name=%s", w.secret.Name)
}

// start begins watching the secret for changes. It runs in an infinite loop,
// automatically reconnecting on errors or connection loss.
// The watcher stops when the context is canceled.
func (w *watcher) start(ctx context.Context) {
	for {
		resourceVersion := w.fetchLatestResourceVersion(ctx)

		watcher, err := w.client.Watch(ctx, metav1.ListOptions{
			FieldSelector:   w.secretNameFieldSelector(),
			ResourceVersion: resourceVersion,
		})
		if err != nil {
			logf.FromContext(ctx).V(1).Info("Error creating watcher. Retrying in 5 seconds...",
				"secret", w.secret.String(),
				"error", err)

			select {
			case <-time.After(reconnectDelay):
			case <-ctx.Done():
				logf.FromContext(ctx).V(1).Info("Context canceled, stopping watcher", "secret", w.secret.String())
				return
			}

			continue
		}

		logf.FromContext(ctx).V(1).Info("Watcher established successfully", "secret", w.secret.String())

		w.processEventLoop(ctx, watcher)

		logf.FromContext(ctx).V(1).Info("Watcher channel closed. Reconnecting in 5 seconds...",
			"secret", w.secret.String())

		metrics.SecretWatcherReconnectsTotal.Inc()

		select {
		case <-time.After(reconnectDelay):
		case <-ctx.Done():
			logf.FromContext(ctx).V(1).Info("Context canceled, stopping watcher", "secret", w.secret.String())
			return
		}
	}
}

// fetchLatestResourceVersion retrieves the most recent resource version for the secret.
// Using List instead of Get ensures we get the latest resourceVersion from the API server,
// which prevents "410 Gone" errors when starting the watch.
// Returns an empty string if the list operation fails.
func (w *watcher) fetchLatestResourceVersion(ctx context.Context) string {
	secretList, err := w.client.List(ctx, metav1.ListOptions{
		FieldSelector: w.secretNameFieldSelector(),
	})
	if err != nil {
		logf.FromContext(ctx).V(1).Info("Could not list secret (it may not exist yet)",
			"secret", w.secret.String(),
			"error", err)
		return ""
	}

	logf.FromContext(ctx).V(1).Info("Fetched latest resource version for secret",
		"secret", w.secret.String(),
		"resourceVersion", secretList.ResourceVersion,
		"exists", len(secretList.Items) > 0)

	return secretList.ResourceVersion
}

// processEventLoop processes watch events until the channel is closed or an error occurs.
func (w *watcher) processEventLoop(ctx context.Context, watcher watch.Interface) {
	for watchEvent := range watcher.ResultChan() {
		if watchEvent.Type == watch.Error {
			logf.FromContext(ctx).V(1).Info("Watch error received",
				"secret", w.secret.String(),
				"object", watchEvent.Object)
			return
		}

		secret, ok := watchEvent.Object.(*corev1.Secret)
		if !ok {
			continue
		}

		metrics.SecretWatchEventsTotal.WithLabelValues(string(watchEvent.Type)).Inc()

		logf.FromContext(ctx).V(1).Info("Secret watch event received. Triggering reconciliation for linked pipelines.",
			"secret", w.secret.String(),
			"eventType", watchEvent.Type,
			"resourceVersion", secret.ResourceVersion,
			"linkedPipelines", len(w.getLinkedPipelines()),
		)

		for _, pipeline := range w.getLinkedPipelines() {
			w.eventRouter(pipeline)
		}
	}
}
