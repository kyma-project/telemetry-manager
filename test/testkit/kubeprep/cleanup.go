package kubeprep

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CleanupCluster removes test resources to prepare for the next test scenario
// This is a best-effort cleanup that logs warnings but doesn't fail
func CleanupCluster(t TestingT, k8sClient client.Client, cfg Config) error {
	t.Helper()
	ctx := t.Context()

	t.Log("Cleaning up cluster resources...")

	// 1. Undeploy telemetry manager (unless skipped)
	if !cfg.SkipManagerDeployment {
		if err := undeployManager(t, k8sClient, cfg); err != nil {
			t.Logf("Warning: failed to undeploy telemetry manager: %v", err)
		}
	}

	// 2. Clean up test prerequisites
	if !cfg.SkipPrerequisites {
		if err := cleanupPrerequisites(t, k8sClient); err != nil {
			t.Logf("Warning: failed to cleanup prerequisites: %v", err)
		}
	}

	// 3. Delete all pipeline CRs
	if err := deletePipelines(ctx, k8sClient, t); err != nil {
		t.Logf("Warning: failed to delete pipelines: %v", err)
	}

	// 4. Delete test namespaces
	if err := deleteTestNamespaces(ctx, k8sClient, t); err != nil {
		t.Logf("Warning: failed to delete test namespaces: %v", err)
	}

	// 5. Remove Istio if installed (regardless of configuration)
	// This ensures the cluster is as close to a fresh k3d cluster as possible
	if isIstioInstalled(ctx, k8sClient) {
		t.Log("Istio detected in cluster, removing...")
		if err := uninstallIstio(t, k8sClient); err != nil {
			t.Logf("Warning: failed to uninstall Istio: %v", err)
		}
	} else {
		t.Log("Istio not detected, skipping Istio cleanup")
	}

	t.Log("Cluster cleanup complete")
	return nil
}

// deletePipelines deletes all LogPipeline, MetricPipeline, and TracePipeline resources
func deletePipelines(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

	t.Log("Deleting pipeline resources...")

	pipelineTypes := []schema.GroupVersionResource{
		{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "logpipelines"},
		{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "metricpipelines"},
		{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "tracepipelines"},
	}

	for _, gvr := range pipelineTypes {
		if err := deleteAllResources(ctx, k8sClient, t, gvr); err != nil {
			t.Logf("Warning: failed to delete %s: %v", gvr.Resource, err)
		}
	}

	return nil
}

// deleteAllResources deletes all resources of a given type
func deleteAllResources(ctx context.Context, k8sClient client.Client, t TestingT, gvr schema.GroupVersionResource) error {
	t.Helper()

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    strings.TrimSuffix(gvr.Resource, "s") + "List",
	})

	if err := k8sClient.List(ctx, list); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to list %s: %w", gvr.Resource, err)
	}

	for _, item := range list.Items {
		if err := k8sClient.Delete(ctx, &item); err != nil && !apierrors.IsNotFound(err) {
			t.Logf("Warning: failed to delete %s %s: %v", gvr.Resource, item.GetName(), err)
		}
	}

	return nil
}

// deleteTestNamespaces deletes namespaces created by tests
func deleteTestNamespaces(ctx context.Context, k8sClient client.Client, t TestingT) error {
	t.Helper()

	t.Log("Deleting test namespaces...")

	namespaces := &corev1.NamespaceList{}
	if err := k8sClient.List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Namespaces to preserve
	preserveNamespaces := map[string]bool{
		"default":               true,
		"kube-system":           true,
		"kube-public":           true,
		"kube-node-lease":       true,
		"kyma-system":           true,
		"istio-system":          true,
		"istio-permissive-mtls": true,
	}

	for _, ns := range namespaces.Items {
		// Skip preserved namespaces
		if preserveNamespaces[ns.Name] {
			continue
		}

		// Delete test namespaces (those with common test prefixes)
		if strings.HasPrefix(ns.Name, "test-") ||
			strings.HasPrefix(ns.Name, "backend-") ||
			strings.HasPrefix(ns.Name, "kyma-integration-") {
			t.Logf("Deleting namespace: %s", ns.Name)
			if err := k8sClient.Delete(ctx, &ns, &client.DeleteOptions{
				GracePeriodSeconds: new(int64), // 0 for immediate deletion
			}); err != nil && !apierrors.IsNotFound(err) {
				t.Logf("Warning: failed to delete namespace %s: %v", ns.Name, err)
			}
		}
	}

	return nil
}

// ForceDeleteNamespace forcefully deletes a namespace by removing finalizers if needed
func ForceDeleteNamespace(t TestingT, k8sClient client.Client, name string) error {
	t.Helper()
	ctx := t.Context()

	ns := &corev1.Namespace{}
	ns.Name = name

	// Try normal delete first
	err := k8sClient.Delete(ctx, ns)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	// If namespace is stuck, remove finalizers
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, ns); err == nil {
		if ns.Status.Phase == corev1.NamespaceTerminating && len(ns.Finalizers) > 0 {
			t.Logf("Removing finalizers from namespace %s", name)
			ns.Finalizers = []string{}
			if err := k8sClient.Update(ctx, ns); err != nil {
				return fmt.Errorf("failed to remove finalizers from namespace %s: %w", name, err)
			}
		}
	}

	return nil
}
