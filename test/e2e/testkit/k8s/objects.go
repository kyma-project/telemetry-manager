//go:build e2e

package k8s

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateObjects creates k8s objects passed as a slice.
func CreateObjects(ctx context.Context, client client.Client, resources ...client.Object) error {
	for _, r := range resources {
		if err := client.Create(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

// DeleteObjects deletes k8s objects passed as a slice.
func DeleteObjects(ctx context.Context, client client.Client, resources ...client.Object) error {
	for _, r := range resources {
		if err := client.Delete(ctx, r); err != nil {
			return err
		}
	}

	return nil
}
