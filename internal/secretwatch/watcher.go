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
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// watcher monitors a single Kubernetes secret and tracks which pipelines depend on it.
// It automatically reconnects on errors and handles resource version updates.
type watcher struct {
	secret    types.NamespacedName
	linked    []client.Object
	mu        sync.RWMutex
	client    v1.SecretInterface
	eventChan chan<- event.GenericEvent
	cancel    context.CancelFunc
}

// newStartedWatcher creates a new watcher for the specified secret with the initial pipeline,
// starts watching in a background goroutine, and returns the fully initialized watcher.
// The watcher will run until the context is canceled.
// The wg.Done() will be called automatically when the watcher stops.
func newStartedWatcher(
	ctx context.Context,
	secret types.NamespacedName,
	pipeline client.Object,
	clientset kubernetes.Interface,
	eventChan chan<- event.GenericEvent,
	wg *sync.WaitGroup,
) *watcher {
	watcherCtx, cancel := context.WithCancel(ctx)

	w := &watcher{
		secret:    secret,
		client:    clientset.CoreV1().Secrets(secret.Namespace),
		eventChan: eventChan,
		cancel:    cancel,
		linked:    []client.Object{pipeline},
	}

	wg.Go(func() {
		w.start(watcherCtx)
	})

	return w
}

// linkedPipelines returns a copy of the linked pipelines for thread-safe access.
func (w *watcher) linkedPipelines() []client.Object {
	w.mu.RLock()
	defer w.mu.RUnlock()

	pipelines := make([]client.Object, len(w.linked))
	copy(pipelines, w.linked)

	return pipelines
}

func (w *watcher) linkPipeline(pipeline client.Object) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if slices.ContainsFunc(w.linked, func(p client.Object) bool {
		return p.GetName() == pipeline.GetName()
	}) {
		return
	}

	w.linked = append(w.linked, pipeline)
}

func (w *watcher) unlinkPipeline(pipeline client.Object) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	filtered := make([]client.Object, 0, len(w.linked))
	for _, p := range w.linked {
		if p.GetName() != pipeline.GetName() {
			filtered = append(filtered, p)
		}
	}

	w.linked = filtered

	return len(w.linked) > 0
}

func (w *watcher) isPipelineLinked(pipeline client.Object) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return slices.ContainsFunc(w.linked, func(p client.Object) bool {
		return p.GetName() == pipeline.GetName()
	})
}

// start begins watching the secret for changes. It runs in an infinite loop,
// automatically reconnecting on errors or connection loss.
// The watcher stops when the context is canceled.
func (w *watcher) start(ctx context.Context) {
	log := logf.FromContext(ctx).V(1)

	for {
		select {
		case <-ctx.Done():
			log.Info("Context canceled, stopping watcher", "secret", w.secret.String())
			return
		default:
		}

		// Get the current resource version to start watching from
		secret, err := w.client.Get(ctx, w.secret.Name, metav1.GetOptions{})

		var resourceVersion string

		if err != nil {
			log.Info("Could not get initial secret (it may not exist yet)",
				"secret", w.secret.String(),
				"error", err)

			resourceVersion = ""
		} else {
			resourceVersion = secret.ResourceVersion
			log.Info("Initial secret found",
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
				log.V(1).Info("Context canceled, stopping watcher", "secret", w.secret.String())
				return
			}

			continue
		}

		log.V(1).Info("Watcher established successfully", "secret", w.secret.String())

		for watchEvent := range watcher.ResultChan() {
			if watchEvent.Type == watch.Error {
				log.V(1).Info("Watch error received",
					"secret", w.secret.String(),
					"object", watchEvent.Object)

				break
			}

			secret, ok := watchEvent.Object.(*corev1.Secret)
			if !ok {
				log.Info("Unexpected object type",
					"secret", w.secret.String(),
					"type", fmt.Sprintf("%T", watchEvent.Object))

				continue
			}

			// Get current linked pipelines for this event
			linkedPipelines := w.linkedPipelines()

			// Log the event
			switch watchEvent.Type {
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
					"eventType", watchEvent.Type,
					"secret", secret.Name,
					"namespace", secret.Namespace,
					"resourceVersion", secret.ResourceVersion,
					"linkedPipelines", linkedPipelines)
			}

			// Send a generic event to trigger reconciliation for linked pipelines
			for _, pipeline := range linkedPipelines {
				w.eventChan <- event.GenericEvent{
					Object: pipeline,
				}
			}
		}

		log.V(1).Info("Watcher channel closed. Reconnecting in 5 seconds...",
			"secret", w.secret.String())

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			log.V(1).Info("Context canceled, stopping watcher", "secret", w.secret.String())
			return
		}
	}
}
