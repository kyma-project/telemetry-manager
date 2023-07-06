//go:build e2e

package k8s

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateObjects creates k8s objects passed as a slice.
func CreateObjects(ctx context.Context, cl client.Client, resources ...client.Object) error {
	for _, r := range resources {
		// Skip object creation if it already exists.
		if labelMatches(r.GetLabels(), "persist", "true") {
			if err := cl.Get(
				ctx,
				types.NamespacedName{Name: r.GetName(), Namespace: r.GetNamespace()},
				reflect.New(reflect.ValueOf(r).Elem().Type()).Interface().(client.Object),
			); err == nil {
				continue
			}
		}

		if err := cl.Create(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

// DeleteObjects deletes k8s objects passed as a slice.
func DeleteObjects(ctx context.Context, client client.Client, resources ...client.Object) error {
	for _, r := range resources {
		// Skip object deletion for persistent ones.
		if labelMatches(r.GetLabels(), "persist", "true") {
			continue
		}
		if err := client.Delete(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

func labelMatches(labels Labels, label, value string) bool {
	l, ok := labels[label]
	if !ok {
		return false
	}

	return l == value
}
