package kubeprep

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensureNamespace creates a namespace if it doesn't exist (internal helper using context directly)
func ensureNamespace(ctx context.Context, k8sClient client.Client, name string, labels map[string]string) error {
	return ensureNamespaceInternal(ctx, k8sClient, name, labels)
}
