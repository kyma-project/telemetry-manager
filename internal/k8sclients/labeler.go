package k8sclients

import (
	"context"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// NewLabeler wraps an existing Kubernetes client to automatically apply the given
// labels to every object on Create, Update, and Patch operations.
// This ensures all managed resources are uniformly labeled and discoverable, which is
// required for scoping the informer cache by label selector.
func NewLabeler(inner client.Client, labels map[string]string) client.Client {
	return interceptor.NewClient(&noopWatchClient{Client: inner}, interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			ensureLabels(obj, labels)
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			ensureLabels(obj, labels)
			return c.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			ensureLabels(obj, labels)
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
}

func ensureLabels(obj client.Object, defaultLabels map[string]string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	maps.Copy(labels, defaultLabels)

	obj.SetLabels(labels)
}
