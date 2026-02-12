package kubeprep

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kymaSystemNamespace      = "kyma-system"
	telemetryManagerName     = "telemetry-manager"
	telemetryReleaseName     = "telemetry"
	telemetryManagerReplicas = 1
)

// getHelmChartPath returns the absolute path to the helm chart
func getHelmChartPath() (string, error) {
	// Try to find go.mod to determine project root
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Walk up the directory tree to find go.mod
	dir := cwd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Found go.mod, helm chart is in <root>/helm
			helmPath := filepath.Join(dir, "helm")
			if _, err := os.Stat(helmPath); err == nil {
				return helmPath, nil
			}
			return "", fmt.Errorf("helm chart not found at %s", helmPath)
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find project root (go.mod)")
}

// deployManager deploys the telemetry manager using helm template
func deployManager(ctx context.Context, k8sClient client.Client, cfg Config) error {
	log.Println("Deploying telemetry manager...")

	// Import local image to k3d if needed
	if cfg.LocalImage {
		clusterName, err := detectK3DCluster(ctx)
		if err != nil {
			log.Printf("Warning: Could not detect k3d cluster: %v", err)
		} else {
			if err := importImageToK3D(ctx, cfg.ManagerImage, clusterName); err != nil {
				return fmt.Errorf("failed to import local image: %w", err)
			}
		}
	}

	// Ensure kyma-system namespace exists
	if err := ensureNamespace(ctx, k8sClient, kymaSystemNamespace, nil); err != nil {
		return fmt.Errorf("failed to ensure kyma-system namespace: %w", err)
	}

	// Get helm chart path
	helmChartPath, err := getHelmChartPath()
	if err != nil {
		return fmt.Errorf("failed to locate helm chart: %w", err)
	}
	log.Printf("Using helm chart at: %s", helmChartPath)

	// Determine pull policy based on image type
	pullPolicy := "Always"
	if cfg.LocalImage {
		pullPolicy = "IfNotPresent"
	}

	// Build helm template command
	// NOTE: The helm chart expects the full image (including tag) in the repository field
	// The deployment template uses: image: {{ .Values.manager.container.image.repository }}
	//
	// IMPORTANT: Set nameOverride=telemetry to match the GitHub workflow deployment.
	// This ensures the fullname template resolves to "telemetry" instead of "telemetry-telemetry-manager".
	// The fullname is used for PriorityClass, webhook configurations, and other resource names.
	args := []string{
		"template",
		telemetryReleaseName,
		helmChartPath,
		"--namespace", kymaSystemNamespace,
		"--set", fmt.Sprintf("experimental.enabled=%t", cfg.EnableExperimental),
		"--set", "default.enabled=true",
		"--set", "nameOverride=telemetry",
		"--set", fmt.Sprintf("manager.container.image.repository=%s", cfg.ManagerImage),
		"--set", fmt.Sprintf("manager.container.image.pullPolicy=%s", pullPolicy),
		"--set", fmt.Sprintf("manager.container.env.operateInFipsMode=%t", cfg.OperateInFIPSMode),
	}

	// Add custom labels/annotations if enabled
	if cfg.CustomLabelsAnnotations {
		args = append(args,
			"--set", "manager.podAnnotations.sidecar\\.istio\\.io/inject=false",
			"--set", "manager.podLabels.custom-pod-label=custom-pod-label-value",
			"--set", "manager.labels.custom-label=custom-label-value",
			"--set", "manager.annotations.custom-annotation=custom-annotation-value",
		)
	}

	log.Printf("Running: helm %v", args)

	// Run helm template command
	cmd := exec.CommandContext(ctx, "helm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm template failed: %w\nstderr: %s", err, stderr.String())
	}

	// Apply the generated YAML
	manifestYAML := stdout.String()
	if err := applyYAML(ctx, k8sClient, manifestYAML); err != nil {
		return fmt.Errorf("failed to apply telemetry manager manifest: %w", err)
	}

	// Wait for telemetry manager deployment to be ready
	log.Println("Waiting for telemetry manager to be ready...")
	if err := waitForManagerReady(ctx, k8sClient); err != nil {
		return fmt.Errorf("telemetry manager not ready: %w", err)
	}

	log.Println("Telemetry manager deployed successfully")
	return nil
}

// waitForManagerReady waits for the telemetry manager deployment to be ready
func waitForManagerReady(ctx context.Context, k8sClient client.Client) error {
	return waitForDeployment(ctx, k8sClient, telemetryManagerName, kymaSystemNamespace, 5*time.Minute)
}

// undeployManager removes the telemetry manager deployment
func undeployManager(ctx context.Context, k8sClient client.Client, cfg Config) error {
	log.Println("Undeploying telemetry manager...")

	// Get helm chart path
	helmChartPath, err := getHelmChartPath()
	if err != nil {
		log.Printf("Warning: failed to locate helm chart: %v", err)
		return nil // Best effort
	}

	// Build helm template command (same as deploy)
	// NOTE: The helm chart expects the full image (including tag) in the repository field
	// The deployment template uses: image: {{ .Values.manager.container.image.repository }}
	//
	// IMPORTANT: Set nameOverride=telemetry to match the GitHub workflow deployment.
	args := []string{
		"template",
		telemetryReleaseName,
		helmChartPath,
		"--namespace", kymaSystemNamespace,
		"--set", fmt.Sprintf("experimental.enabled=%t", cfg.EnableExperimental),
		"--set", "default.enabled=true",
		"--set", "nameOverride=telemetry",
		"--set", fmt.Sprintf("manager.container.image.repository=%s", cfg.ManagerImage),
		"--set", "manager.container.image.pullPolicy=Always",
		"--set", fmt.Sprintf("manager.container.env.operateInFipsMode=%t", cfg.OperateInFIPSMode),
	}

	if cfg.CustomLabelsAnnotations {
		args = append(args,
			"--set", "manager.podAnnotations.sidecar\\.istio\\.io/inject=false",
			"--set", "manager.podLabels.custom-pod-label=custom-pod-label-value",
			"--set", "manager.labels.custom-label=custom-label-value",
			"--set", "manager.annotations.custom-annotation=custom-annotation-value",
		)
	}

	// Run helm template command
	cmd := exec.CommandContext(ctx, "helm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Warning: helm template failed during undeploy: %v\nstderr: %s", err, stderr.String())
		return nil // Best effort
	}

	// Delete the generated YAML
	manifestYAML := stdout.String()
	if err := deleteYAML(ctx, k8sClient, manifestYAML); err != nil {
		log.Printf("Warning: failed to delete telemetry manager manifest: %v", err)
		return nil // Best effort
	}

	log.Println("Telemetry manager undeployed")
	return nil
}
