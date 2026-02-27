package kubeprep

import (
	"bytes"
	"context"
	"os/exec"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IstioState represents the current state of Istio installation
type IstioState int

const (
	// IstioNotInstalled means no Istio CR exists
	IstioNotInstalled IstioState = iota
	// IstioFullyInstalled means Istio CR exists, manager is running, and istiod is ready
	IstioFullyInstalled
	// IstioOrphaned means Istio CR exists but manager is not running (CR stuck with finalizer)
	IstioOrphaned
	// IstioPartiallyInstalled means Istio CR exists and manager is running but istiod is not ready
	IstioPartiallyInstalled
)

// String returns a human-readable description of the Istio state
func (s IstioState) String() string {
	switch s {
	case IstioNotInstalled:
		return "not installed"
	case IstioFullyInstalled:
		return "fully installed"
	case IstioOrphaned:
		return "orphaned (CR exists but no manager)"
	case IstioPartiallyInstalled:
		return "partially installed (manager running but istiod not ready)"
	default:
		return "unknown"
	}
}

// NeedsReinstall returns true if Istio needs to be reinstalled before it can be used
func (s IstioState) NeedsReinstall() bool {
	return s == IstioOrphaned || s == IstioPartiallyInstalled
}

// DetectIstioState checks the current state of Istio installation.
// It examines: Istio CR existence, Istio manager deployment, and istiod deployment.
func DetectIstioState(ctx context.Context, k8sClient client.Client) IstioState {
	if k8sClient == nil {
		return IstioNotInstalled
	}

	// Check if the Istio CR exists
	if !istioCRExists(ctx, k8sClient) {
		return IstioNotInstalled
	}

	// CR exists - check if Istio manager is running
	if !isIstioManagerRunning(ctx, k8sClient) {
		return IstioOrphaned
	}

	// Manager is running - check if istiod is ready
	if !isIstiodReady(ctx, k8sClient) {
		return IstioPartiallyInstalled
	}

	return IstioFullyInstalled
}

// istioCRExists checks if the default Istio CR exists in kyma-system namespace
func istioCRExists(ctx context.Context, k8sClient client.Client) bool {
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

	return err == nil
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

// getReleaseValues returns the current user-supplied values for the helm release as a YAML string.
// Returns empty string if the release doesn't exist or detection fails.
func getReleaseValues(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "helm", "get", "values", telemetryReleaseName, "-n", kymaSystemNamespace, "-o", "yaml")

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	return stdout.String()
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

// isIstioManagerRunning checks if the Istio manager deployment exists and is ready
func isIstioManagerRunning(ctx context.Context, k8sClient client.Client) bool {
	deployment := &appsv1.Deployment{}

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "istio-controller-manager",
		Namespace: kymaSystemNamespace,
	}, deployment)
	if err != nil {
		return false
	}

	// Check if deployment is ready
	return deployment.Spec.Replicas != nil &&
		deployment.Status.ReadyReplicas >= 1 &&
		deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
}
