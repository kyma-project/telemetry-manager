package k8s

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// CreateObjects creates k8s objects passed as a slice.
func CreateObjects(t *testing.T, resources ...client.Object) error {
	t.Helper()

	// Sort resources:
	// 1. namespaces
	// 2. other resources
	// 3. pipelines
	sortedResources := sortObjects(resources)

	t.Cleanup(func() {
		gomega.Expect(DeleteObjects(resources...)).To(gomega.Succeed())
	})

	for _, resource := range sortedResources {
		// Skip object creation if it already exists.
		if labelMatches(resource.GetLabels(), kitk8sobjects.PersistentLabelName, "true") {
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

		// assert object readiness
		switch r := resource.(type) {
		case *appsv1.Deployment:
			assert.DeploymentReady(t, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
		case *appsv1.DaemonSet:
			assert.DaemonSetReady(t, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
		case *appsv1.StatefulSet:
			assert.StatefulSetReady(t, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
		case *corev1.Pod:
			assert.PodReady(t, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
		}
	}

	return nil
}

func sortObjects(resources []client.Object) []client.Object {
	return slices.SortedFunc(slices.Values(resources), func(a, b client.Object) int {
		var (
			isNamespaceA, isNamespaceB, isPipelineA, isPipelineB bool
		)

		switch a.(type) {
		case *telemetryv1alpha1.MetricPipeline, *telemetryv1alpha1.TracePipeline, *telemetryv1alpha1.LogPipeline:
			isPipelineA = true
		case *telemetryv1beta1.MetricPipeline, *telemetryv1beta1.TracePipeline, *telemetryv1beta1.LogPipeline:
			isPipelineA = true
		case *corev1.Namespace:
			isNamespaceA = true
		}

		switch b.(type) {
		case *telemetryv1alpha1.MetricPipeline, *telemetryv1alpha1.TracePipeline, *telemetryv1alpha1.LogPipeline:
			isPipelineB = true
		case *telemetryv1beta1.MetricPipeline, *telemetryv1beta1.TracePipeline, *telemetryv1beta1.LogPipeline:
			isPipelineB = true
		case *corev1.Namespace:
			isNamespaceB = true
		}

		if isNamespaceA && !isNamespaceB {
			return -1
		}

		if !isNamespaceA && isNamespaceB {
			return 1
		}

		if isPipelineA && !isPipelineB {
			return 1
		}

		if !isPipelineA && isPipelineB {
			return -1
		}

		return 0
	},
	)
}

// DeleteObjects deletes k8s objects passed as a slice.
// This function directly uses context.Background(), since in go test the context gets canceled before deletion sometimes,
func DeleteObjects(resources ...client.Object) error {
	for _, r := range resources {
		// Skip object deletion for persistent ones.
		if labelMatches(r.GetLabels(), kitk8sobjects.PersistentLabelName, "true") {
			continue
		}

		if err := suite.K8sClient.Delete(context.Background(), r); err != nil {
			return err
		}
	}

	return nil
}

// ForceDeleteObjects deletes k8s objects including persistent ones.
func ForceDeleteObjects(t *testing.T, resources ...client.Object) error {
	for _, r := range resources {
		if err := suite.K8sClient.Delete(t.Context(), r); err != nil {
			return err
		}
	}

	return nil
}

// UpdateObjects updates k8s objects passed as a slice.
func UpdateObjects(t *testing.T, resources ...client.Object) error {
	for _, resource := range resources {
		if err := suite.K8sClient.Update(t.Context(), resource); err != nil {
			return err
		}
	}

	return nil
}

// ObjectsToFile retrieves k8s objects, cleans them up and writes them to a YAML file.
func ObjectsToFile(t *testing.T, resources ...client.Object) error {
	t.Helper()

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	for _, resource := range resources {
		err := suite.K8sClient.Get(t.Context(), types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}, resource)
		if err != nil {
			return err
		}

		resource.SetManagedFields(nil)
		resource.SetOwnerReferences(nil)
		resource.SetCreationTimestamp(metav1.Time{})
		resource.SetUID(``)
		resource.SetDeletionTimestamp(nil)
		resource.SetDeletionGracePeriodSeconds(nil)
		resource.SetResourceVersion("")

		if err = enc.Encode(resource); err != nil {
			return err
		}
	}

	if err := enc.Close(); err != nil {
		return err
	}

	return os.WriteFile(strings.ReplaceAll(t.Name(), "/", "_")+".yaml", buf.Bytes(), 0600)
}

func labelMatches(labels kitk8sobjects.Labels, label, value string) bool {
	l, ok := labels[label]
	if !ok {
		return false
	}

	return l == value
}
