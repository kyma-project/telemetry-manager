package k8sclients

import (
	"context"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

// NewLabeler wraps an existing Kubernetes client to automatically apply default
// telemetry labels to every object on Create, Update, and Patch operations.
// This ensures all managed resources are uniformly labeled and discoverable, which is
// required for scoping the informer cache by label selector.
//
// The following labels are applied:
//   - kyma-project.io/module: telemetry
//   - app.kubernetes.io/part-of: telemetry
//   - app.kubernetes.io/managed-by: telemetry-manager
//   - app.kubernetes.io/name: <baseName>
//   - app.kubernetes.io/component: <componentType>
func NewLabeler(inner client.Client, baseName, componentType string) client.Client {
	defaultLabels := commonresources.MakeDefaultLabels(baseName, componentType)

	return interceptor.NewClient(&noopWatchClient{Client: inner}, interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			ensureDefaultLabels(obj, defaultLabels)
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			ensureDefaultLabels(obj, defaultLabels)
			return c.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			ensureDefaultLabels(obj, defaultLabels)
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
}

func ensureDefaultLabels(obj client.Object, defaultLabels map[string]string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	maps.Copy(labels, defaultLabels)

	obj.SetLabels(labels)
}
