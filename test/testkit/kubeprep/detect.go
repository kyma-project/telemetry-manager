package kubeprep

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DetectIstioInstalled checks if Istio is fully installed and operational.
// It verifies both the Istio CR exists AND istiod deployment is ready.
// This prevents false positives from partial installations where the CR exists
// but istiod failed to start.
func DetectIstioInstalled(ctx context.Context, k8sClient client.Client) bool {
	if k8sClient == nil {
		return false
	}

	// Check if the default Istio CR exists in kyma-system namespace
	// This is the CR created by our Istio installation: istios.operator.kyma-project.io/default
	istioCR := &unstructured.Unstructured{}
	istioCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "default",
		Namespace: "kyma-system",
	}, istioCR)
	if err != nil {
		return false
	}

	// Also verify istiod deployment is ready to ensure Istio is fully operational
	// This catches partial installations where the CR exists but istiod failed to start
	return isIstiodReady(ctx, k8sClient)
}

// detectExperimentalEnabled checks if the current deployment has experimental mode enabled
// by inspecting the Helm release values.
// Returns false if the release doesn't exist or detection fails.
func detectExperimentalEnabled(ctx context.Context) bool {
	return detectExperimentalFromHelm(ctx)
}

// detectExperimentalFromHelm checks the helm release to see if experimental is enabled
func detectExperimentalFromHelm(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "helm", "get", "values", telemetryReleaseName, "-n", kymaSystemNamespace, "-o", "json")

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false
	}

	// Simple check: look for experimental.enabled in the output
	output := stdout.String()

	return strings.Contains(output, `"experimental":{"enabled":true}`) ||
		strings.Contains(output, `"experimental": {"enabled": true}`)
}

// releaseExists checks if the helm release exists
func releaseExists(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "helm", "status", telemetryReleaseName, "-n", kymaSystemNamespace)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Release doesn't exist or helm command failed
		return false
	}

	return true
}

// isIstiodReady checks if the istiod deployment exists and is ready
func isIstiodReady(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "istiod",
		Namespace: istioNamespace,
	}, deployment)
	if err != nil {
		return false
	}

	// Check if deployment is ready (has at least one ready replica)
	return deployment.Spec.Replicas != nil &&
		deployment.Status.ReadyReplicas >= 1 &&
		deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
}

// DetectIstioPartiallyInstalled checks if Istio is in a partial installation state
// (CR exists but istiod is not ready). This indicates a failed installation that
// should be cleaned up before retrying.
func DetectIstioPartiallyInstalled(ctx context.Context, k8sClient client.Client) bool {
	if k8sClient == nil {
		return false
	}

	// Check if the Istio CR exists
	istioCR := &unstructured.Unstructured{}
	istioCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "default",
		Namespace: "kyma-system",
	}, istioCR)
	if err != nil {
		// CR doesn't exist, so not partially installed
		return false
	}

	// CR exists - check if istiod is NOT ready (partial installation)
	return !isIstiodReady(ctx, k8sClient)
}

// DetectOrphanedIstioCR checks if there's an Istio CR without a running Istio manager.
// This can happen when the Istio manager was removed while the CR still has a finalizer,
// leaving the CR stuck and unable to be deleted.
func DetectOrphanedIstioCR(ctx context.Context, k8sClient client.Client) bool {
	if k8sClient == nil {
		return false
	}

	// Check if the Istio CR exists
	istioCR := &unstructured.Unstructured{}
	istioCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "default",
		Namespace: "kyma-system",
	}, istioCR)
	if err != nil {
		// CR doesn't exist
		return false
	}

	// CR exists - check if Istio manager is NOT running
	return !isIstioManagerRunning(ctx, k8sClient)
}

// isIstioManagerRunning checks if the Istio manager deployment exists and is ready
func isIstioManagerRunning(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "istio-controller-manager",
		Namespace: "istio-system",
	}, deployment)
	if err != nil {
		return false
	}

	// Check if deployment is ready
	return deployment.Spec.Replicas != nil &&
		deployment.Status.ReadyReplicas >= 1 &&
		deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
}
