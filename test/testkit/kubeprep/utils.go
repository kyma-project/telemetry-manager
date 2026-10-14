package kubeprep

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// applyFromURL fetches YAML content from a URL and applies it to the cluster
func applyFromURL(ctx context.Context, k8sClient client.Client, url string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx // URL is from trusted sources (Istio/Kyma repos)
	if err != nil {
		return fmt.Errorf("failed to fetch YAML from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch YAML from %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return applyYAML(ctx, k8sClient, string(body))
}

// applyYAML parses YAML content and applies each object to the cluster using server-side apply.
// This creates new resources or updates existing ones.
func applyYAML(ctx context.Context, k8sClient client.Client, yamlContent string) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(yamlContent), 4096)

	for {
		obj := &unstructured.Unstructured{}

		err := decoder.Decode(obj)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		// Skip empty objects
		if len(obj.Object) == 0 {
			continue
		}

		// Use server-side apply to create or update the object
		err = k8sClient.Patch(ctx, obj, client.Apply, client.FieldOwner("kubeprep"), client.ForceOwnership) //nolint:staticcheck // client.Apply is the standard way for server-side apply
		if err != nil {
			return fmt.Errorf("failed to apply %s %s/%s: %w",
				obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}

// deleteYAML parses YAML content and deletes each object from the cluster
func deleteYAML(ctx context.Context, k8sClient client.Client, yamlContent string) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(yamlContent), 4096)

	for {
		obj := &unstructured.Unstructured{}

		err := decoder.Decode(obj)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		// Skip empty objects
		if len(obj.Object) == 0 {
			continue
		}

		// Delete the object (ignore not found errors)
		err = k8sClient.Delete(ctx, obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete %s %s/%s: %w",
				obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}

// deleteFromURL fetches YAML content from a URL and deletes it from the cluster
func deleteFromURL(ctx context.Context, k8sClient client.Client, url string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx // URL is from trusted sources (Istio/Kyma repos)
	if err != nil {
		return fmt.Errorf("failed to fetch YAML from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch YAML from %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return deleteYAML(ctx, k8sClient, string(body))
}

// deleteNamespace deletes a namespace (best effort, ignores not found)
func deleteNamespace(ctx context.Context, k8sClient client.Client, name string) error {
	ns := &corev1.Namespace{}
	ns.Name = name

	err := k8sClient.Delete(ctx, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	return nil
}

// deleteAllResourcesByGVRK deletes all resources of a given GroupVersionResourceKind across all namespaces
func deleteAllResourcesByGVRK(ctx context.Context, k8sClient client.Client, group, version, resource, kind string) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind + "List",
	})

	// List all resources (across all namespaces)
	if err := k8sClient.List(ctx, list); err != nil {
		// CRD may not exist - handle various "not found" error types:
		// - IsNotFound: resource not found
		// - IsMethodNotSupported: method not supported
		// - "no matches for kind": CRD not registered in scheme/discovery
		if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) || isNoKindMatchError(err) {
			return nil
		}

		return fmt.Errorf("failed to list %s.%s: %w", resource, group, err)
	}

	// Delete each resource
	for _, item := range list.Items {
		obj := item.DeepCopy()
		if err := k8sClient.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}

// countResourcesByGVRK counts all resources of a given GroupVersionResourceKind
func countResourcesByGVRK(ctx context.Context, k8sClient client.Client, group, version, resource, kind string) (int, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind + "List",
	})

	// List all resources (across all namespaces)
	if err := k8sClient.List(ctx, list); err != nil {
		// CRD may not exist - handle various "not found" error types
		if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) || isNoKindMatchError(err) {
			return 0, nil
		}

		return 0, fmt.Errorf("failed to list %s.%s: %w", resource, group, err)
	}

	return len(list.Items), nil
}

// isNotFoundError checks if an error is a NotFound error
func isNotFoundError(err error) bool {
	return apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err)
}

// isNoKindMatchError checks if an error is a "no matches for kind" error.
// This occurs when the CRD is not registered in the cluster.
func isNoKindMatchError(err error) bool {
	if err == nil {
		return false
	}
	// The error message contains "no matches for kind" when the CRD doesn't exist
	return strings.Contains(err.Error(), "no matches for kind")
}

// waitForDeployment waits for a deployment to be ready
func waitForDeployment(ctx context.Context, k8sClient client.Client, name, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		deployment := &appsv1.Deployment{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, deployment)
		if err != nil {
			if apierrors.IsNotFound(err) {
				time.Sleep(5 * time.Second)
				continue
			}

			return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
		}

		// Check if deployment is ready
		if deployment.Spec.Replicas != nil &&
			deployment.Status.ReadyReplicas == *deployment.Spec.Replicas &&
			*deployment.Spec.Replicas > 0 {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for deployment %s/%s to be ready", namespace, name)
}

// waitForDeploymentDeletion waits for a deployment to be fully deleted
func waitForDeploymentDeletion(ctx context.Context, k8sClient client.Client, t TestingT, name, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		deployment := &appsv1.Deployment{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, deployment)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
		}

		t.Logf("Waiting for deployment %s/%s to be deleted...", namespace, name)
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for deployment %s/%s to be deleted", namespace, name)
}

// ensureNamespaceInternal creates a namespace if it doesn't exist (internal implementation)
func ensureNamespaceInternal(ctx context.Context, k8sClient client.Client, name string, labels map[string]string) error {
	ns := &corev1.Namespace{}

	err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, ns)
	if err == nil {
		// Namespace exists, update labels if needed
		if labels != nil {
			if ns.Labels == nil {
				ns.Labels = make(map[string]string)
			}

			maps.Copy(ns.Labels, labels)

			return k8sClient.Update(ctx, ns)
		}

		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get namespace %s: %w", name, err)
	}

	// Create namespace
	ns = &corev1.Namespace{}
	ns.Name = name
	ns.Labels = labels

	err = k8sClient.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	return nil
}

// crdExists checks if a CRD exists in the cluster
func crdExists(ctx context.Context, k8sClient client.Client, name string) (bool, error) {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, crd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
