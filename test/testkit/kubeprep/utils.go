package kubeprep

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// loadEnvFile reads a .env file and returns a map of key-value pairs
func loadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			env[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .env file: %w", err)
	}

	return env, nil
}

// applyFromURL fetches YAML content from a URL and applies it to the cluster
func applyFromURL(ctx context.Context, k8sClient client.Client, t TestingT, url string) error {
	resp, err := http.Get(url)
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

	return applyYAML(ctx, k8sClient, t, string(body))
}

// applyYAML parses YAML content and applies each object to the cluster using server-side apply.
// This creates new resources or updates existing ones.
func applyYAML(ctx context.Context, k8sClient client.Client, t TestingT, yamlContent string) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(yamlContent), 4096)

	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		// Skip empty objects
		if obj.Object == nil || len(obj.Object) == 0 {
			continue
		}

		// Use server-side apply to create or update the object
		err = k8sClient.Patch(ctx, obj, client.Apply, client.FieldOwner("kubeprep"), client.ForceOwnership)
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
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		// Skip empty objects
		if obj.Object == nil || len(obj.Object) == 0 {
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
	resp, err := http.Get(url)
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
		if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
			return nil
		}
		return fmt.Errorf("failed to list %s.%s: %w", resource, group, err)
	}

	// Delete each resource
	for _, item := range list.Items {
		obj := item.DeepCopy()
		_ = k8sClient.Delete(ctx, obj)
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
		if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
			// CRD might not exist or not be installed
			return 0, nil
		}
		return 0, fmt.Errorf("failed to list %s.%s: %w", resource, group, err)
	}

	return len(list.Items), nil
}

// deleteAllResourcesByGVR deletes all resources of a given GroupVersionResource across all namespaces
// Deprecated: Use deleteAllResourcesByGVRK instead, which uses the correct Kind
func deleteAllResourcesByGVR(ctx context.Context, k8sClient client.Client, group, version, resource string) error {
	kind := strings.TrimSuffix(resource, "s")
	return deleteAllResourcesByGVRK(ctx, k8sClient, group, version, resource, kind)
}

// countResourcesByGVR counts all resources of a given GroupVersionResource
// Deprecated: Use countResourcesByGVRK instead, which uses the correct Kind
func countResourcesByGVR(ctx context.Context, k8sClient client.Client, group, version, resource string) (int, error) {
	// Try to guess the Kind from the resource name (may not work for all cases)
	kind := strings.TrimSuffix(resource, "s")
	return countResourcesByGVRK(ctx, k8sClient, group, version, resource, kind)
}

// isNotFoundError checks if an error is a NotFound error
func isNotFoundError(err error) bool {
	return apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err)
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

// waitForServiceAccount waits for a service account to exist
func waitForServiceAccount(ctx context.Context, k8sClient client.Client, name, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		sa := &corev1.ServiceAccount{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, sa)

		if err == nil {
			return nil
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get service account %s/%s: %w", namespace, name, err)
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for service account %s/%s", namespace, name)
}

// retryWithBackoff retries a function with exponential backoff
func retryWithBackoff(ctx context.Context, maxAttempts int, delay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}
	return fmt.Errorf("max attempts (%d) exceeded: %w", maxAttempts, lastErr)
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
			for k, v := range labels {
				ns.Labels[k] = v
			}
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

// convertToTyped converts an unstructured object to a typed object
func convertToTyped(obj *unstructured.Unstructured, typedObj runtime.Object) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, typedObj)
}
