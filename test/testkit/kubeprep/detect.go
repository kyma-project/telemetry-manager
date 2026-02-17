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

// detectExperimentalEnabled checks if the current deployment has experimental mode enabled.
// It checks if the experimental CRD fields are present (e.g., LogPipeline with OTLPInput).
// Returns false if manager is not deployed or detection fails.
func detectExperimentalEnabled(ctx context.Context, k8sClient client.Client) bool {
	if k8sClient == nil {
		return false
	}

	// Check for the manager deployment and look at its configuration
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      telemetryManagerName,
		Namespace: kymaSystemNamespace,
	}, deployment)

	if err != nil {
		return false // Manager not deployed, assume non-experimental
	}

	// Check if the experimental LogPipeline CRD exists with OTLPInput field
	// The experimental subchart has different CRD versions
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})

	err = k8sClient.Get(ctx, types.NamespacedName{
		Name: "logpipelines.telemetry.kyma-project.io",
	}, crd)

	if err != nil {
		return false
	}

	// Check the CRD spec to see if it has experimental fields
	// The experimental CRD has additional input types like OTLPInput
	spec, found, _ := unstructured.NestedMap(crd.Object, "spec", "versions")
	if !found || spec == nil {
		// Try to detect by looking at helm release values
		return detectExperimentalFromHelm(ctx)
	}

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
func releaseExists(ctx context.Context, t TestingT) bool {
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
