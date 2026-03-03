package kubeprep

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kymaSystemNamespace  = "kyma-system"
	telemetryReleaseName = "telemetry"

	// telemetryManagerName is the name of the telemetry manager deployment
	telemetryManagerName = "telemetry-manager"

	// reconcileDelay is the time to wait after upgrade for the manager to reconcile resources
	reconcileDelay = 30 * time.Second
)

// waitForRolloutComplete waits until the deployment rollout is complete,
// meaning only the new replica is running and no old replicas remain.
func waitForRolloutComplete(ctx context.Context, k8sClient client.Client, t TestingT, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		deployment := &appsv1.Deployment{}
		if err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      telemetryManagerName,
			Namespace: kymaSystemNamespace,
		}, deployment); err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}

		// Check rollout status:
		// - UpdatedReplicas equals desired replicas (new pods created)
		// - ReadyReplicas equals desired replicas (all pods ready)
		// - AvailableReplicas equals desired replicas (all pods available)
		// - No old replicas remaining (Replicas == UpdatedReplicas)
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}

		if deployment.Status.UpdatedReplicas == desired &&
			deployment.Status.ReadyReplicas == desired &&
			deployment.Status.AvailableReplicas == desired &&
			deployment.Status.Replicas == desired {
			t.Log("Deployment rollout complete")
			return nil
		}

		t.Logf("Waiting for rollout: updated=%d, ready=%d, available=%d, total=%d (desired=%d)",
			deployment.Status.UpdatedReplicas,
			deployment.Status.ReadyReplicas,
			deployment.Status.AvailableReplicas,
			deployment.Status.Replicas,
			desired)

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for deployment rollout to complete")
}

// waitForSinglePod waits until exactly one pod with the given label is running
func waitForSinglePod(ctx context.Context, k8sClient client.Client, t TestingT, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pods := &corev1.PodList{}
		if err := k8sClient.List(ctx, pods,
			client.InNamespace(kymaSystemNamespace),
			client.MatchingLabels{"control-plane": "telemetry-manager"},
		); err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}

		runningCount := 0

		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil {
				runningCount++
			}
		}

		if runningCount == 1 {
			t.Log("Single manager pod running")
			return nil
		}

		t.Logf("Waiting for single pod: %d running pods found", runningCount)
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for single manager pod")
}

// getHelmChartPath returns the absolute path to the local helm chart
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

// deployManagerFromChartSource deploys the telemetry manager from a helm chart using helm upgrade --install.
// The chartSource can be either a local file path or a remote URL.
// If imageOverride is non-empty, it overrides the image in the chart.
func deployManagerFromChartSource(t TestingT, k8sClient client.Client, chartSource string, imageOverride string, cfg Config) error {
	ctx := t.Context()

	// Import local image to k3d if needed
	if imageOverride != "" && IsLocalImage(imageOverride) {
		if clusterName, err := detectK3DCluster(ctx); err == nil {
			if err := importImageToK3D(ctx, t, imageOverride, clusterName); err != nil {
				t.Logf("Warning: failed to import image to k3d: %v", err)
			}
		}
	}

	// Ensure kyma-system namespace exists
	if err := ensureNamespace(ctx, k8sClient, kymaSystemNamespace, nil); err != nil {
		return fmt.Errorf("failed to ensure kyma-system namespace: %w", err)
	}

	// Build helm upgrade --install command
	// Note: experimental and default are mutually exclusive subcharts
	args := []string{
		"upgrade", "--install",
		telemetryReleaseName,
		chartSource,
		"--namespace", kymaSystemNamespace,
		"--set", fmt.Sprintf("experimental.enabled=%t", cfg.EnableExperimental),
		"--set", fmt.Sprintf("default.enabled=%t", !cfg.EnableExperimental),
		"--set", "nameOverride=telemetry",
		"--set", fmt.Sprintf("manager.container.env.operateInFipsMode=%t", cfg.OperateInFIPSMode),
		"--wait",
		"--timeout", "5m",
	}

	// Override image if specified
	if imageOverride != "" {
		pullPolicy := "Always"
		if IsLocalImage(imageOverride) {
			pullPolicy = "IfNotPresent"
		}

		args = append(args,
			"--set", fmt.Sprintf("manager.container.image.repository=%s", imageOverride),
			"--set", fmt.Sprintf("manager.container.image.pullPolicy=%s", pullPolicy),
		)
	}

	// Add custom helm values if provided
	for _, helmValue := range cfg.HelmValues {
		args = append(args, "--set", helmValue)
	}

	// Run helm upgrade --install command
	t.Logf("Running: helm %v", args)
	cmd := exec.CommandContext(ctx, "helm", args...)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm upgrade --install failed: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}

// deployManager deploys the telemetry manager using helm upgrade --install.
// If cfg.ChartPath is set, uses that chart with its baked-in image (for deploying released versions).
// Otherwise, uses the local helm chart with cfg.ManagerImage override.
func deployManager(t TestingT, k8sClient client.Client, cfg Config) error {
	chartSource := cfg.ChartPath
	imageOverride := cfg.ManagerImage

	if chartSource == "" {
		// Using local chart - need to override with target image
		var err error

		chartSource, err = getHelmChartPath()
		if err != nil {
			return fmt.Errorf("failed to locate helm chart: %w", err)
		}
	} else {
		// Using released chart - use the image baked into the chart
		imageOverride = ""
	}

	t.Logf("Deploying telemetry manager from: %s", chartSource)

	if err := deployManagerFromChartSource(t, k8sClient, chartSource, imageOverride, cfg); err != nil {
		return err
	}

	t.Log("Telemetry manager deployed")

	return nil
}

// buildHelmValues builds a map of helm values that will be applied during deployment.
func buildHelmValues(cfg Config, imageOverride string) map[string]any {
	values := map[string]any{
		"experimental": map[string]any{
			"enabled": cfg.EnableExperimental,
		},
		"default": map[string]any{
			"enabled": !cfg.EnableExperimental,
		},
		"nameOverride": "telemetry",
		"manager": map[string]any{
			"container": map[string]any{
				"env": map[string]any{
					"operateInFipsMode": cfg.OperateInFIPSMode,
				},
			},
		},
	}

	// Add image override if specified
	if imageOverride != "" {
		pullPolicy := "Always"
		if IsLocalImage(imageOverride) {
			pullPolicy = "IfNotPresent"
		}

		if managerMap, ok := values["manager"].(map[string]any); ok {
			if containerMap, ok := managerMap["container"].(map[string]any); ok {
				containerMap["image"] = map[string]any{
					"repository": imageOverride,
					"pullPolicy": pullPolicy,
				}
			}
		}
	}

	// Apply custom helm values (--set key=value format)
	for _, helmValue := range cfg.HelmValues {
		applyHelmSetValue(values, helmValue)
	}

	return values
}

// applyHelmSetValue applies a single --set key=value to the values map.
// Supports nested keys like "manager.container.image.repository=foo".
func applyHelmSetValue(values map[string]any, setValue string) {
	parts := strings.SplitN(setValue, "=", 2)
	if len(parts) != 2 {
		return
	}

	key, value := parts[0], parts[1]
	keyParts := strings.Split(key, ".")

	// Navigate to the parent map, creating intermediate maps as needed
	current := values

	for i := range len(keyParts) - 1 {
		part := keyParts[i]
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			next := make(map[string]any)
			current[part] = next
			current = next
		}
	}

	// Set the final value (try to preserve type for booleans)
	finalKey := keyParts[len(keyParts)-1]

	switch value {
	case "true":
		current[finalKey] = true
	case "false":
		current[finalKey] = false
	default:
		current[finalKey] = value
	}
}

// valuesEqual compares current helm values (YAML string) with new values (map).
// Returns true if they represent the same configuration.
func valuesEqual(currentYAML string, newValues map[string]any) bool {
	if currentYAML == "" {
		// No current release, so values are definitely different
		return false
	}

	var currentValues map[string]any
	if err := yaml.Unmarshal([]byte(currentYAML), &currentValues); err != nil {
		// Can't parse current values, assume different
		return false
	}

	// Sort both maps recursively and compare as YAML
	sortMapRecursively(currentValues)
	sortMapRecursively(newValues)

	currentSorted, err1 := yaml.Marshal(currentValues)
	newSorted, err2 := yaml.Marshal(newValues)

	if err1 != nil || err2 != nil {
		return false
	}

	return string(currentSorted) == string(newSorted)
}

// sortMapRecursively sorts all nested maps in place by their keys.
// This ensures deterministic YAML output for comparison.
func sortMapRecursively(m map[string]any) {
	for k, v := range m {
		if nested, ok := v.(map[string]any); ok {
			sortMapRecursively(nested)
			m[k] = sortedMap(nested)
		}
	}
}

// sortedMap returns a new map with keys in sorted order.
// yaml.v3 preserves insertion order, so we rebuild the map with sorted keys.
func sortedMap(m map[string]any) map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sorted := make(map[string]any, len(m))
	for _, k := range keys {
		sorted[k] = m[k]
	}

	return sorted
}

// undeployManager removes the telemetry manager following the proper cleanup order:
// 1. Delete all pipeline resources (LogPipeline, TracePipeline, MetricPipeline)
// 2. Delete the Telemetry operator CR
// 3. Wait for resources to be deleted
// 4. Uninstall the helm chart
// 5. Wait for manager deployment to be deleted
func undeployManager(t TestingT, k8sClient client.Client) error {
	ctx := t.Context()

	t.Log("Undeploying telemetry manager...")

	// Step 1: Delete all pipeline resources
	t.Log("Deleting all pipeline resources...")

	if err := deleteTelemetryPipelines(ctx, k8sClient); err != nil {
		return fmt.Errorf("failed to delete telemetry pipelines: %w", err)
	}

	// Wait for pipelines to be deleted
	if err := waitForPipelinesDeletion(ctx, k8sClient, t); err != nil {
		return fmt.Errorf("failed waiting for pipelines deletion: %w", err)
	}

	// Step 2: Delete the Telemetry CR
	t.Log("Deleting Telemetry CR...")

	if err := deleteTelemetryCR(ctx, k8sClient); err != nil {
		return fmt.Errorf("failed to delete Telemetry CR: %w", err)
	}

	// Wait for Telemetry CR to be deleted
	if err := waitForTelemetryCRDeletion(ctx, k8sClient, t); err != nil {
		return fmt.Errorf("failed waiting for Telemetry CR deletion: %w", err)
	}

	// Step 3: Uninstall the helm chart
	t.Log("Uninstalling helm release...")

	args := []string{
		"uninstall",
		telemetryReleaseName,
		"--namespace", kymaSystemNamespace,
		"--wait",
		"--timeout", "2m",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Ignore errors - release might not exist
		t.Logf("Warning: helm uninstall failed (release may not exist): %v", err)
	}

	// Step 4: Wait for manager deployment to be fully deleted
	t.Log("Waiting for manager deployment to be deleted...")

	if err := waitForDeploymentDeletion(ctx, k8sClient, t, telemetryManagerName, kymaSystemNamespace, 2*time.Minute); err != nil {
		t.Logf("Warning: failed waiting for manager deployment deletion: %v", err)
	}

	t.Log("Telemetry manager undeployed")

	return nil
}

// deleteTelemetryPipelines deletes all LogPipeline, TracePipeline, and MetricPipeline resources
func deleteTelemetryPipelines(ctx context.Context, k8sClient client.Client) error {
	// Delete LogPipelines
	if err := deleteAllResourcesByGVRK(ctx, k8sClient, "telemetry.kyma-project.io", "v1alpha1", "logpipelines", "LogPipeline"); err != nil {
		return fmt.Errorf("failed to delete LogPipelines: %w", err)
	}

	// Delete TracePipelines
	if err := deleteAllResourcesByGVRK(ctx, k8sClient, "telemetry.kyma-project.io", "v1alpha1", "tracepipelines", "TracePipeline"); err != nil {
		return fmt.Errorf("failed to delete TracePipelines: %w", err)
	}

	// Delete MetricPipelines
	if err := deleteAllResourcesByGVRK(ctx, k8sClient, "telemetry.kyma-project.io", "v1alpha1", "metricpipelines", "MetricPipeline"); err != nil {
		return fmt.Errorf("failed to delete MetricPipelines: %w", err)
	}

	return nil
}

// deleteTelemetryCR deletes the Telemetry operator CR
func deleteTelemetryCR(ctx context.Context, k8sClient client.Client) error {
	return deleteAllResourcesByGVRK(ctx, k8sClient, "operator.kyma-project.io", "v1beta1", "telemetries", "Telemetry")
}

// waitForPipelinesDeletion waits for all pipeline resources to be deleted
func waitForPipelinesDeletion(ctx context.Context, k8sClient client.Client, t TestingT) error {
	maxAttempts := 30
	delay := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		totalCount := 0

		// Count LogPipelines
		count, err := countResourcesByGVRK(ctx, k8sClient, "telemetry.kyma-project.io", "v1alpha1", "logpipelines", "LogPipeline")
		if err != nil {
			return fmt.Errorf("failed to count LogPipelines: %w", err)
		}

		totalCount += count

		// Count TracePipelines
		count, err = countResourcesByGVRK(ctx, k8sClient, "telemetry.kyma-project.io", "v1alpha1", "tracepipelines", "TracePipeline")
		if err != nil {
			return fmt.Errorf("failed to count TracePipelines: %w", err)
		}

		totalCount += count

		// Count MetricPipelines
		count, err = countResourcesByGVRK(ctx, k8sClient, "telemetry.kyma-project.io", "v1alpha1", "metricpipelines", "MetricPipeline")
		if err != nil {
			return fmt.Errorf("failed to count MetricPipelines: %w", err)
		}

		totalCount += count

		if totalCount == 0 {
			t.Log("All pipeline resources deleted")
			return nil
		}

		t.Logf("Waiting for pipeline deletion: %d resources remaining", totalCount)
		time.Sleep(delay)
	}

	return fmt.Errorf("timeout: pipeline resources still exist after %d attempts", maxAttempts)
}

// waitForTelemetryCRDeletion waits for the Telemetry CR to be deleted
func waitForTelemetryCRDeletion(ctx context.Context, k8sClient client.Client, t TestingT) error {
	maxAttempts := 60
	delay := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		count, err := countResourcesByGVRK(ctx, k8sClient, "operator.kyma-project.io", "v1beta1", "telemetries", "Telemetry")
		if err != nil {
			if isNotFoundError(err) {
				t.Log("Telemetry CR deleted")
				return nil
			}
		}

		if count == 0 {
			t.Log("Telemetry CR deleted")
			return nil
		}

		t.Logf("Waiting for Telemetry CR deletion: %d resources remaining", count)
		time.Sleep(delay)
	}

	return fmt.Errorf("timeout: Telemetry CR still exists after %d attempts", maxAttempts)
}
