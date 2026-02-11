package secretwatch

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// watcher monitors a single Kubernetes secret and tracks which pipelines depend on it.
// It automatically reconnects on errors and handles resource version updates.
type watcher struct {
	secret          types.NamespacedName
	linkedPipelines []string
	mu              sync.RWMutex
	client          typedcorev1.SecretInterface
	eventHandler    EventHandler
}

// newWatcher creates a new watcher for the specified secret.
// It initializes the watcher with the given linked pipelines and a Kubernetes client
// for the secret's namespace.
func newWatcher(secret types.NamespacedName, linkedPipelines []string, clientset kubernetes.Interface, eventHandler EventHandler) *watcher {
	return &watcher{
		secret:          secret,
		linkedPipelines: linkedPipelines,
		client:          clientset.CoreV1().Secrets(secret.Namespace),
		eventHandler:    eventHandler,
	}
}

// getLinkedPipelines returns a copy of the linked pipelines for thread-safe access.
func (w *watcher) getLinkedPipelines() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	pipelines := make([]string, len(w.linkedPipelines))
	copy(pipelines, w.linkedPipelines)
	return pipelines
}

// addPipeline adds a pipeline to the linked pipelines list if not already present.
// It is thread-safe and can be called concurrently.
func (w *watcher) addPipeline(pipelineName string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if slices.Contains(w.linkedPipelines, pipelineName) {
		return
	}

	w.linkedPipelines = append(w.linkedPipelines, pipelineName)
}

// removePipeline removes a pipeline from the linked pipelines list.
// It returns true if any pipelines remain after removal.
// It is thread-safe and can be called concurrently.
func (w *watcher) removePipeline(pipelineName string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	filtered := make([]string, 0, len(w.linkedPipelines))
	for _, p := range w.linkedPipelines {
		if p != pipelineName {
			filtered = append(filtered, p)
		}
	}
	w.linkedPipelines = filtered
	return len(w.linkedPipelines) > 0
}

// hasPipeline checks if a pipeline is in the linked pipelines list.
// It is thread-safe and can be called concurrently.
func (w *watcher) hasPipeline(pipelineName string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return slices.Contains(w.linkedPipelines, pipelineName)
}

// Start begins watching the secret for changes. It runs in an infinite loop,
// automatically reconnecting on errors or connection loss.
// The watcher stops when the context is cancelled.
func (w *watcher) Start(ctx context.Context) {
	log := logf.FromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			log.V(1).Info("Context cancelled, stopping watcher", "secret", w.secret.String())
			return
		default:
		}

		// Get the current resource version to start watching from
		secret, err := w.client.Get(ctx, w.secret.Name, metav1.GetOptions{})
		var resourceVersion string
		if err != nil {
			log.V(1).Info("Could not get initial secret (it may not exist yet)",
				"secret", w.secret.String(),
				"error", err)
			resourceVersion = ""
		} else {
			resourceVersion = secret.ResourceVersion
			log.V(1).Info("Initial secret found",
				"secret", w.secret.String(),
				"resourceVersion", resourceVersion)
		}

		// Create a watcher for the specific secret
		watcher, err := w.client.Watch(ctx, metav1.ListOptions{
			FieldSelector:   fmt.Sprintf("metadata.name=%s", w.secret.Name),
			ResourceVersion: resourceVersion,
		})
		if err != nil {
			log.V(1).Info("Error creating watcher. Retrying in 5 seconds...",
				"secret", w.secret.String(),
				"error", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				log.V(1).Info("Context cancelled, stopping watcher", "secret", w.secret.String())
				return
			}
			continue
		}

		log.V(1).Info("Watcher established successfully", "secret", w.secret.String())

		for event := range watcher.ResultChan() {
			if event.Type == watch.Error {
				log.V(1).Info("Watch error received",
					"secret", w.secret.String(),
					"object", event.Object)
				break
			}

			secret, ok := event.Object.(*corev1.Secret)
			if !ok {
				log.Info("Unexpected object type",
					"secret", w.secret.String(),
					"type", fmt.Sprintf("%T", event.Object))
				continue
			}

			// Get current linked pipelines for this event
			linkedPipelines := w.getLinkedPipelines()

			// Log the event
			switch event.Type {
			case watch.Added:
				log.Info("Secret added",
					"secret", secret.Name,
					"namespace", secret.Namespace,
					"resourceVersion", secret.ResourceVersion,
					"linkedPipelines", linkedPipelines)
			case watch.Modified:
				log.Info("Secret modified",
					"secret", secret.Name,
					"namespace", secret.Namespace,
					"resourceVersion", secret.ResourceVersion,
					"linkedPipelines", linkedPipelines)
			case watch.Deleted:
				log.Info("Secret deleted",
					"secret", secret.Name,
					"namespace", secret.Namespace,
					"resourceVersion", secret.ResourceVersion,
					"linkedPipelines", linkedPipelines)
			default:
				log.Info("Secret event",
					"eventType", event.Type,
					"secret", secret.Name,
					"namespace", secret.Namespace,
					"resourceVersion", secret.ResourceVersion,
					"linkedPipelines", linkedPipelines)
			}

			// Call event handler if provided
			if w.eventHandler != nil {
				w.eventHandler(w.secret, event.Type, linkedPipelines)
			}
		}

		log.V(1).Info("Watcher channel closed. Reconnecting in 5 seconds...",
			"secret", w.secret.String())
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			log.V(1).Info("Context cancelled, stopping watcher", "secret", w.secret.String())
			return
		}
	}
}
