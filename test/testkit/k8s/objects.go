package k8s

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// CreateObjects creates k8s objects passed as a slice.
func CreateObjects(t *testing.T, resources ...client.Object) error {
	t.Helper()

	t.Cleanup(func() {
		// Delete created objects after test completion. We dont care for not found errors here.
		gomega.Expect(deleteObjectsIgnoringNotFound(resources...)).To(gomega.Succeed())
		gomega.Eventually(allObjectsDeleted(resources...), periodic.EventuallyTimeout).Should(gomega.Succeed())
	})

	return createObjects(t, resources...)
}

// CreateObjectsWithoutAutomaticCleanup creates k8s objects passed as a slice but does not delete them automatically after the test.
func CreateObjectsWithoutAutomaticCleanup(t *testing.T, resources ...client.Object) error {
	t.Helper()

	return createObjects(t, resources...)
}

func createObjects(t *testing.T, resources ...client.Object) error {
	t.Helper()

	mixed, pipelines := sortObjects(resources)

	for _, resource := range mixed {
		err := createObject(t, resource)
		if err != nil {
			return err
		}
	}

	// wait for all deployments, daemonsets, statefulsets, pods to be ready before applying pipelines
	for _, resource := range mixed {
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

	// apply pipeline objects
	for _, resource := range pipelines {
		err := createObject(t, resource)
		if err != nil {
			return err
		}
	}

	return nil
}

// createObject creates a single k8s object.
// If the object has the persistent label, it checks if the object already exists and skips creation if it does.
func createObject(t *testing.T, resource client.Object) error {
	// Skip creation for persistent objects if they already exist.
	if hasPersistentLabel(resource.GetLabels()) {
		//nolint:errcheck // The value is guaranteed to be of type client.Object.
		existingResource := reflect.New(reflect.ValueOf(resource).Elem().Type()).Interface().(client.Object)
		err := suite.K8sClient.Get(
			t.Context(),
			types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()},
			existingResource,
		)
		// If the object is not found, proceed to create it.
		if err == nil {
			return nil
		}

		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	return suite.K8sClient.Create(t.Context(), resource)
}

func allObjectsDeleted(resources ...client.Object) error {
	for _, r := range resources {
		if hasPersistentLabel(r.GetLabels()) {
			continue
		}

		err := suite.K8sClient.Get(context.Background(), types.NamespacedName{Name: r.GetName(), Namespace: r.GetNamespace()}, r)
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func sortObjects(resources []client.Object) ([]client.Object, []client.Object) {
	// Split sorted resources into three slices: namespaces, pipelines, others
	var (
		namespaces []client.Object
		pipelines  []client.Object
		others     []client.Object
	)

	for _, r := range resources {
		switch r.(type) {
		case *corev1.Namespace:
			namespaces = append(namespaces, r)
		case *telemetryv1alpha1.MetricPipeline, *telemetryv1alpha1.TracePipeline, *telemetryv1alpha1.LogPipeline,
			*telemetryv1beta1.MetricPipeline, *telemetryv1beta1.TracePipeline, *telemetryv1beta1.LogPipeline:
			pipelines = append(pipelines, r)
		default:
			others = append(others, r)
		}
	}

	return append(namespaces, others...), pipelines
}

// DeleteObjects deletes k8s objects passed as a slice.
// This function directly uses context.Background(), since in go test the context gets canceled before deletion sometimes,
func DeleteObjects(resources ...client.Object) error {
	for _, r := range resources {
		// Skip object deletion for persistent ones.
		if hasPersistentLabel(r.GetLabels()) {
			continue
		}

		if err := suite.K8sClient.Delete(context.Background(), r); err != nil {
			return err
		}
	}

	return nil
}

func deleteObjectsIgnoringNotFound(resources ...client.Object) error {
	for _, r := range resources {
		// Skip object deletion for persistent ones.
		if hasPersistentLabel(r.GetLabels()) {
			continue
		}

		if err := client.IgnoreNotFound(suite.K8sClient.Delete(context.Background(), r)); err != nil {
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

func hasPersistentLabel(labels kitk8sobjects.Labels) bool {
	return labelMatches(labels, kitk8sobjects.PersistentLabelName, "true")
}

func resetTelemetryResource(t *testing.T, previous operatorv1alpha1.Telemetry) {
	t.Helper()
	gomega.Eventually(func(g gomega.Gomega) {
		var current operatorv1alpha1.Telemetry
		g.Expect(suite.K8sClient.Get(context.Background(), types.NamespacedName{Namespace: previous.Namespace, Name: previous.Name}, &current)).NotTo(gomega.HaveOccurred())
		current.Spec = previous.Spec
		current.Labels = previous.Labels
		current.Annotations = previous.Annotations
		g.Expect(suite.K8sClient.Update(context.Background(), &current)).NotTo(gomega.HaveOccurred(), "should reset Telemetry resource to previous state")
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(gomega.Succeed())
}

func PreserveAndScheduleRestoreOfTelemetryResource(t *testing.T, key types.NamespacedName) {
	t.Helper()

	var previous operatorv1alpha1.Telemetry
	gomega.Expect(suite.K8sClient.Get(t.Context(), key, &previous)).NotTo(gomega.HaveOccurred())
	t.Cleanup(func() {
		resetTelemetryResource(t, previous)
	})
}
