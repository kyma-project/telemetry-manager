package k8sclients

import (
	"context"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

// NewLabeler wraps an existing Kubernetes client to automatically apply common
// telemetry module labels to every object on Create, Update, and Patch operations.
// This ensures all managed resources are uniformly labeled and discoverable, which is
// required for scoping the informer cache by label selector.
//
// The following labels are applied:
//   - kyma-project.io/module: telemetry
//   - app.kubernetes.io/part-of: telemetry
//   - app.kubernetes.io/managed-by: telemetry-manager
func NewLabeler(inner client.Client) client.Client {
	return interceptor.NewClient(&noopWatchClient{Client: inner}, interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			ensureModuleLabels(obj)
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			ensureModuleLabels(obj)
			return c.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			ensureModuleLabels(obj)
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
}

func ensureModuleLabels(obj client.Object) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	maps.Copy(labels, commonresources.MakeModuleLabels())

	obj.SetLabels(labels)
}
