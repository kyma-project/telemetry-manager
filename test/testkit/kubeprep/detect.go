package kubeprep

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DetectIstioInstalled checks if Istio is installed by looking for the default Istio CR.
// This is the only detection needed because Istio changes require special handling
// (manager must be removed before Istio changes).
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

	return err == nil
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
