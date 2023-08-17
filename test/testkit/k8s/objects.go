package k8s

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateObjects creates k8s objects passed as a slice.
func CreateObjects(ctx context.Context, cl client.Client, resources ...client.Object) error {
	for _, resource := range resources {
		// Skip object creation if it already exists.
		if labelMatches(resource.GetLabels(), PersistentLabelName, "true") {
			//nolint:errcheck // The value is guaranteed to be of type client.Object.
			existingResource := reflect.New(reflect.ValueOf(resource).Elem().Type()).Interface().(client.Object)
			if err := cl.Get(
				ctx,
				types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()},
				existingResource,
			); err == nil {
				if versionsMatch(resource, existingResource) {
					continue
				}

				if err = cl.Delete(ctx, existingResource); err != nil {
					return err
				}
			}
		}

		if err := cl.Create(ctx, resource); err != nil {
			return err
		}
	}

	return nil
}

// DeleteObjects deletes k8s objects passed as a slice.
func DeleteObjects(ctx context.Context, client client.Client, resources ...client.Object) error {
	for _, r := range resources {
		// Skip object deletion for persistent ones.
		if labelMatches(r.GetLabels(), PersistentLabelName, "true") {
			continue
		}
		if err := client.Delete(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

// ForceDeleteObjects deletes k8s objects including persistent ones.
func ForceDeleteObjects(ctx context.Context, client client.Client, resources ...client.Object) error {
	for _, r := range resources {
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

func versionsMatch(new, existing client.Object) bool {
	newVersion := new.GetLabels()[VersionLabelName]
	existingVersion, ok := existing.GetLabels()[VersionLabelName]
	if !ok {
		return true
	}

	return newVersion == existingVersion
}
