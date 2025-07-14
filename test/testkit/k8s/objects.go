package k8s

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// CreateObjects creates k8s objects passed as a slice.
func CreateObjects(t testkit.T, resources ...client.Object) error {
	t.Helper()

	for _, resource := range resources {
		// Skip object creation if it already exists.
		if labelMatches(resource.GetLabels(), PersistentLabelName, "true") {
			//nolint:errcheck // The value is guaranteed to be of type client.Object.
			existingResource := reflect.New(reflect.ValueOf(resource).Elem().Type()).Interface().(client.Object)
			if err := suite.K8sClient.Get(
				t.Context(),
				types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()},
				existingResource,
			); err == nil {
				continue
			}
		}

		if err := suite.K8sClient.Create(t.Context(), resource); err != nil {
			return err
		}
	}

	return nil
}

// DeleteObjects deletes k8s objects passed as a slice.
// This function directly uses context.Background(), since in go test the context gets canceled before deletion sometimes,
func DeleteObjects(resources ...client.Object) error {
	for _, r := range resources {
		// Skip object deletion for persistent ones.
		if labelMatches(r.GetLabels(), PersistentLabelName, "true") {
			continue
		}

		if err := suite.K8sClient.Delete(context.Background(), r); err != nil {
			return err
		}
	}

	return nil
}

// ForceDeleteObjects deletes k8s objects including persistent ones.
func ForceDeleteObjects(t testkit.T, resources ...client.Object) error {
	for _, r := range resources {
		if err := suite.K8sClient.Delete(t.Context(), r); err != nil {
			return err
		}
	}

	return nil
}

// UpdateObjects updates k8s objects passed as a slice.
func UpdateObjects(t testkit.T, resources ...client.Object) error {
	for _, resource := range resources {
		if err := suite.K8sClient.Update(t.Context(), resource); err != nil {
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
