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
	kubecoreev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

// unlink removes a pipeline from the watcher's linked pipelines.
// It returns true if there are still pipelines linked after removal, or false if the list is now empty.
func (w *watcher) unlink(pipeline client.Object) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.linked = slices.DeleteFunc(w.linked, func(p client.Object) bool {
		return samePipeline(p, pipeline)
	})

	return len(w.linked) > 0
}

func (w *watcher) isLinked(pipeline client.Object) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return slices.ContainsFunc(w.linked, func(p client.Object) bool {
		return samePipeline(p, pipeline)
	})
}

func (w *watcher) getLinkedPipelines() []client.Object {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return slices.Clone(w.linked)
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
			case <-time.After(reconnectDelay):
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
				continue
			}

			log.Info("Secret watch event received. Triggering reconciliation for linked pipelines.",
				"secret", w.secret.String(),
				"eventType", watchEvent.Type,
				"resourceVersion", secret.ResourceVersion,
				"linkedPipelines", len(w.getLinkedPipelines()),
			)

			// Send events to trigger reconciliation for linked pipelines
			for _, pipeline := range w.getLinkedPipelines() {
				w.eventRouter(pipeline)
			}
		}

		log.V(1).Info("Watcher channel closed. Reconnecting in 5 seconds...",
			"secret", w.secret.String())

		select {
		case <-time.After(reconnectDelay):
		case <-ctx.Done():
			log.V(1).Info("Context canceled, stopping watcher", "secret", w.secret.String())
			return
		}
	}
}
